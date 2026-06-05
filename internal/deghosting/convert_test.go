package deghosting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Unverified/deghosting/internal/deghosting"
	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

func TestConvertExportConvertsPublishedPosts(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "published-posts.json")

	result, err := deghosting.ConvertExport(export)
	require.NoError(t, err)
	require.ElementsMatch(t, []deghosting.SkippedPost{
		{Slug: "draft", Reason: "not published"},
		{Slug: "about", Reason: "not a post"},
	}, result.Skipped)
	require.Empty(t, result.Warnings)
	require.Len(t, result.Posts, 1)

	post := result.Posts[0]
	require.Equal(t, "hello-world", post.Slug)
	require.Contains(t, post.Body, "**Ghost**")
	require.Equal(t, zola.FrontMatter{
		Title:       `Hello "World"`,
		Date:        post.FrontMatter.Date,
		Description: "A greeting.",
		Authors:     []string{"Ada Lovelace", "Grace Hopper"},
		Taxonomies: zola.Taxonomies{
			Tags: []string{"Announcements", "Adopters"},
		},
		Extra: zola.Extra{
			FeatureImage: "__GHOST_URL__/feature.png",
		},
	}, post.FrontMatter)
	require.Equal(t, "2020-05-19T12:03:00Z", post.FrontMatter.Date.Format("2006-01-02T15:04:05Z07:00"))
	// Assets are populated in the resolve+plan step; empty until then.
	require.Empty(t, post.Assets)
}

func TestConvertExportUsesMetaDescriptionFallback(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "meta-description-fallback.json")

	result, err := deghosting.ConvertExport(export)
	require.NoError(t, err)
	require.Len(t, result.Posts, 1)
	require.Equal(t, "SEO fallback.", result.Posts[0].FrontMatter.Description)
	require.Empty(t, result.Posts[0].Assets)
}

func TestConvertExportSkipsPublishedPostWithoutPublishedAt(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "missing-published-at.json")

	result, err := deghosting.ConvertExport(export)
	require.NoError(t, err)
	require.Empty(t, result.Posts)
	require.Equal(t, []deghosting.Warning{
		{PostSlug: "missing-date", Message: "published post without published_at, skipping"},
	}, result.Warnings)
}

func TestConvertExportRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "invalid-slug.json")

	result, err := deghosting.ConvertExport(export)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid post slug")
	require.Empty(t, result.Posts)
}

func parseExportFixture(t *testing.T, name string) *ghost.Export {
	t.Helper()

	input := openFixture(t, name)
	defer func() { _ = input.Close() }()

	export, err := ghost.Parse(input)
	require.NoError(t, err)

	return export
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", name))
	require.NoError(t, err)

	return file
}
