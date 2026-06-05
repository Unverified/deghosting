// Package deghosting converts Ghost exports into generated Zola posts.
package deghosting

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
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
func Convert(input io.Reader) (ConvertResult, error) {
	export, err := ghost.Parse(input)
	if err != nil {
		return ConvertResult{}, err
	}

	return ConvertExport(export)
}

// ConvertExport converts published Ghost posts from export into Zola posts.
func ConvertExport(export *ghost.Export) (ConvertResult, error) {
	if export == nil || len(export.DB) != 1 {
		return ConvertResult{}, fmt.Errorf("convert ghost export: expected exactly one db entry")
	}

	converter := exportConverter{
		data: export.DB[0].Data,
	}
	converter.index()

	return converter.convert()
}

type exportConverter struct {
	data ghost.Data

	tagsByID        map[string]ghost.Tag
	usersByID       map[string]ghost.User
	metaByPostID    map[string]ghost.PostMeta
	tagsByPostID    map[string][]ghost.PostTag
	authorsByPostID map[string][]ghost.PostAuthor
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

		zolaPost, err := c.convertPost(post)
		if err != nil {
			return result, err
		}

		result.Posts = append(result.Posts, zolaPost)
	}
	return result, nil
}

func (c *exportConverter) convertPost(post ghost.Post) (zola.Post, error) {
	if err := validateSlug(post.Slug); err != nil {
		return zola.Post{}, err
	}

	body, err := htmltomarkdown.ConvertString(post.HTML)
	if err != nil {
		return zola.Post{}, fmt.Errorf("convert HTML for %q: %w", post.Slug, err)
	}

	res := zola.Post{
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
				FeatureImage: post.FeatureImage,
			},
		},
		Body: strings.TrimSpace(body),
	}
	return res, nil
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
