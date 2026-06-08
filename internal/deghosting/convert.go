// Package deghosting converts Ghost exports into generated Zola posts.
package deghosting

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
	"golang.org/x/net/html"
)

// ConvertResult summarizes one conversion run.
type ConvertResult struct {
	Posts    []zola.Post
	Skipped  []SkippedPost
	Warnings []Warning
}

// SkippedPost describes an expected post that did not enter the content tree.
type SkippedPost struct {
	Slug   string
	Reason string
}

func (s SkippedPost) String() string {
	if s.Slug == "" {
		return s.Reason
	}
	return fmt.Sprintf("%s: %s", s.Slug, s.Reason)
}

// Warning describes a non-fatal conversion issue.
type Warning struct {
	PostSlug string
	Message  string
}

func (w Warning) String() string {
	if w.PostSlug == "" {
		return w.Message
	}
	return fmt.Sprintf("%s: %s", w.PostSlug, w.Message)
}

// Convert parses a Ghost export from input and converts it to Zola posts.
// ghostURL is the site origin that replaces the __GHOST_URL__ placeholder in
// asset references; it is required only when Ghost-hosted asset references are
// present.
func Convert(input io.Reader, ghostURL string) (ConvertResult, error) {
	export, err := ghost.Parse(input)
	if err != nil {
		return ConvertResult{}, err
	}

	return ConvertExport(export, ghostURL)
}

// ConvertExport converts published Ghost posts from export into Zola posts.
func ConvertExport(export *ghost.Export, ghostURL string) (ConvertResult, error) {
	if export == nil || len(export.DB) != 1 {
		return ConvertResult{}, fmt.Errorf("convert ghost export: expected exactly one db entry")
	}

	var base *url.URL
	if ghostURL != "" {
		var err error
		base, err = parseGhostOrigin(ghostURL)
		if err != nil {
			return ConvertResult{}, fmt.Errorf("convert ghost export: invalid ghost URL %q: %w", ghostURL, err)
		}
	}

	converter := exportConverter{
		data: export.DB[0].Data,
		base: base,
	}
	converter.index()

	return converter.convert()
}

type exportConverter struct {
	data ghost.Data
	base *url.URL // parsed Ghost URL; nil when --ghost-url was not provided

	tagsByID        map[string]ghost.Tag
	usersByID       map[string]ghost.User
	metaByPostID    map[string]ghost.PostMeta
	tagsByPostID    map[string][]ghost.PostTag
	authorsByPostID map[string][]ghost.PostAuthor
	convertedSlugs  map[string]struct{}
}

func (c *exportConverter) index() {
	c.tagsByID = make(map[string]ghost.Tag, len(c.data.Tags))
	for _, tag := range c.data.Tags {
		c.tagsByID[tag.ID] = tag
	}

	c.usersByID = make(map[string]ghost.User, len(c.data.Users))
	for _, user := range c.data.Users {
		c.usersByID[user.ID] = user
	}

	c.metaByPostID = make(map[string]ghost.PostMeta, len(c.data.PostsMeta))
	for _, meta := range c.data.PostsMeta {
		c.metaByPostID[meta.PostID] = meta
	}

	c.tagsByPostID = make(map[string][]ghost.PostTag)
	for _, postTag := range c.data.PostsTags {
		c.tagsByPostID[postTag.PostID] = append(c.tagsByPostID[postTag.PostID], postTag)
	}

	c.authorsByPostID = make(map[string][]ghost.PostAuthor)
	for _, postAuthor := range c.data.PostsAuthors {
		c.authorsByPostID[postAuthor.PostID] = append(c.authorsByPostID[postAuthor.PostID], postAuthor)
	}

	c.convertedSlugs = make(map[string]struct{})
	for _, post := range c.data.Posts {
		if post.Type != "post" || post.Status != "published" || post.PublishedAt.IsZero() {
			continue
		}
		if validateSlug(post.Slug) != nil {
			continue
		}
		c.convertedSlugs[post.Slug] = struct{}{}
	}
}

func (c *exportConverter) convert() (ConvertResult, error) {
	var result ConvertResult

	for _, post := range c.data.Posts {
		if post.Type != "post" {
			result.Skipped = append(result.Skipped, SkippedPost{
				Slug:   post.Slug,
				Reason: "not a post",
			})
			continue
		}

		if post.Status != "published" {
			result.Skipped = append(result.Skipped, SkippedPost{
				Slug:   post.Slug,
				Reason: "not published",
			})
			continue
		}

		if post.PublishedAt.IsZero() {
			result.Warnings = append(result.Warnings, Warning{
				PostSlug: post.Slug,
				Message:  "published post without published_at, skipping",
			})
			continue
		}

		zolaPost, warnings, err := c.convertPost(post)
		if err != nil {
			return result, err
		}

		result.Warnings = append(result.Warnings, warnings...)
		result.Posts = append(result.Posts, zolaPost)
	}
	return result, nil
}

func (c *exportConverter) convertPost(post ghost.Post) (zola.Post, []Warning, error) {
	if err := validateSlug(post.Slug); err != nil {
		return zola.Post{}, nil, err
	}

	rewrittenHTML, assets, err := processAssets(post, c.base)
	if err != nil {
		return zola.Post{}, nil, err
	}

	rewrittenHTML, linkWarnings := rewriteBodyLinks(
		post.Slug,
		rewrittenHTML,
		c.base,
		c.convertedSlugs,
	)

	remoteToLocal := make(map[string]string, len(assets))
	for _, a := range assets {
		remoteToLocal[a.RemoteURL] = a.LocalPath
	}

	body, err := convertBodyHTML(rewrittenHTML)
	if err != nil {
		return zola.Post{}, nil, fmt.Errorf("convert HTML for %q: %w", post.Slug, err)
	}

	return zola.Post{
		Slug: post.Slug,
		FrontMatter: zola.FrontMatter{
			Title:       post.Title,
			Date:        post.PublishedAt.Time,
			Description: c.description(post),
			Authors:     c.authorNames(post.ID),
			Taxonomies: zola.Taxonomies{
				Tags: c.tagNames(post.ID),
			},
			Extra: zola.Extra{
				FeatureImage: lookupLocal(post.FeatureImage, c.base, remoteToLocal),
				OGImage:      lookupLocal(post.OGImage, c.base, remoteToLocal),
				TwitterImage: lookupLocal(post.TwitterImage, c.base, remoteToLocal),
			},
		},
		Body:   strings.TrimSpace(body),
		Assets: assets,
	}, linkWarnings, nil
}

func convertBodyHTML(input string) (string, error) {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)
	conv.Register.RendererFor("audio", converter.TagTypeBlock, base.RenderAsHTML, converter.PriorityEarly)
	conv.Register.RendererFor("video", converter.TagTypeBlock, base.RenderAsHTML, converter.PriorityEarly)
	conv.Register.RendererFor("figure", converter.TagTypeBlock, renderMediaFigureHTML, converter.PriorityEarly)

	return conv.ConvertString(input)
}

func renderMediaFigureHTML(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	if !containsNativeMedia(node) {
		return converter.RenderTryNext
	}
	return base.RenderAsHTML(ctx, w, node)
}

func containsNativeMedia(node *html.Node) bool {
	if node.Type == html.ElementNode && (node.Data == "audio" || node.Data == "video") {
		return true
	}
	for child := range node.ChildNodes() {
		if containsNativeMedia(child) {
			return true
		}
	}
	return false
}

func (c *exportConverter) description(post ghost.Post) string {
	if post.CustomExcerpt != "" {
		return post.CustomExcerpt
	}
	return c.metaByPostID[post.ID].MetaDescription
}

func (c *exportConverter) tagNames(postID string) []string {
	joins := slices.Clone(c.tagsByPostID[postID])
	slices.SortFunc(joins, func(a, b ghost.PostTag) int {
		return a.SortOrder - b.SortOrder
	})

	names := make([]string, 0, len(joins))
	for _, join := range joins {
		tag, ok := c.tagsByID[join.TagID]
		if !ok || tag.Name == "" {
			continue
		}
		names = append(names, tag.Name)
	}

	return names
}

func (c *exportConverter) authorNames(postID string) []string {
	joins := slices.Clone(c.authorsByPostID[postID])
	slices.SortFunc(joins, func(a, b ghost.PostAuthor) int {
		return a.SortOrder - b.SortOrder
	})

	names := make([]string, 0, len(joins))
	for _, join := range joins {
		user, ok := c.usersByID[join.AuthorID]
		if !ok || user.Name == "" {
			continue
		}
		names = append(names, user.Name)
	}

	return names
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("convert ghost export: post slug is empty")
	}
	if slug == "." || slug == ".." || strings.Contains(slug, "/") || strings.Contains(slug, `\`) {
		return fmt.Errorf("convert ghost export: invalid post slug %q", slug)
	}
	if filepath.IsAbs(slug) || !filepath.IsLocal(slug) {
		return fmt.Errorf("convert ghost export: invalid post slug %q", slug)
	}

	return nil
}
