package deghosting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Unverified/deghosting/internal/deghosting"
	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/stretchr/testify/require"
)

const convertTestGhostURL = "https://blog.example.com"

func TestConvertExportConvertsPublishedPosts(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "published-posts.json")

	result, err := deghosting.ConvertExport(export, convertTestGhostURL)
	require.NoError(t, err)
	require.ElementsMatch(t, []deghosting.SkippedPost{
		{Slug: "draft", Reason: "not published"},
		{Slug: "about", Reason: "not a post"},
	}, result.Skipped)
	require.Empty(t, result.Warnings)
	require.Len(t, result.Posts, 1)

	post := result.Posts[0]
	require.Equal(t, "hello-world", post.Slug)
	require.Equal(t, "2020-05-19T12:03:00Z", post.FrontMatter.Date.Format("2006-01-02T15:04:05Z07:00"))
	require.Equal(t, `Hello "World"`, post.FrontMatter.Title)
	require.Equal(t, "A greeting.", post.FrontMatter.Description)
	require.Equal(t, []string{"Ada Lovelace", "Grace Hopper"}, post.FrontMatter.Authors)
	require.Equal(t, []string{"Announcements", "Adopters"}, post.FrontMatter.Taxonomies.Tags)

	// Ghost-hosted extras are rewritten to local paths.
	require.Regexp(t, `^[0-9a-f]{8}-feature\.png$`, post.FrontMatter.Extra.FeatureImage)
	require.Regexp(t, `^[0-9a-f]{8}-og\.png$`, post.FrontMatter.Extra.OGImage)
	require.Regexp(t, `^[0-9a-f]{8}-twitter\.png$`, post.FrontMatter.Extra.TwitterImage)
}

func TestConvertExportRewritesBodyImages(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "published-posts.json")

	result, err := deghosting.ConvertExport(export, convertTestGhostURL)
	require.NoError(t, err)
	require.Len(t, result.Posts, 1)

	post := result.Posts[0]

	// Ghost-hosted body image is rewritten to a local path; placeholder is gone.
	require.NotContains(t, post.Body, "__GHOST_URL__")
	require.Regexp(t, `[0-9a-f]{8}-body-1\.png`, post.Body)

	// External body image is left untouched.
	require.Contains(t, post.Body, "https://example.com/body-2.jpg")

	// Body still has its text content.
	require.Contains(t, post.Body, "**Ghost**")
}

func TestConvertExportPopulatesAssets(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "published-posts.json")

	result, err := deghosting.ConvertExport(export, convertTestGhostURL)
	require.NoError(t, err)
	require.Len(t, result.Posts, 1)

	post := result.Posts[0]

	// 4 Ghost-hosted refs: feature, og, twitter, body-1. body-2 is external.
	require.Len(t, post.Assets, 4)
	for _, a := range post.Assets {
		require.NotEmpty(t, a.RemoteURL)
		require.Regexp(t, `^[0-9a-f]{8}-`, a.LocalPath)
	}
}

func TestConvertExportRejectsInvalidGhostURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ghostURL string
	}{
		{name: "missing scheme", ghostURL: "blog.example.com"},
		{name: "unsupported scheme", ghostURL: "ftp://blog.example.com"},
		{name: "path", ghostURL: "https://blog.example.com/content"},
		{name: "query", ghostURL: "https://blog.example.com?preview=true"},
		{name: "fragment", ghostURL: "https://blog.example.com#images"},
		{name: "user info", ghostURL: "https://user:pass@blog.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			export := parseExportFixture(t, "published-posts.json")
			result, err := deghosting.ConvertExport(export, tc.ghostURL)

			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid ghost URL")
			require.Empty(t, result.Posts)
		})
	}
}

func TestConvertExportAcceptsGhostOriginWithTrailingSlash(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "published-posts.json")

	result, err := deghosting.ConvertExport(export, "https://blog.example.com/")

	require.NoError(t, err)
	require.Len(t, result.Posts, 1)
	require.Len(t, result.Posts[0].Assets, 4)
}

func TestConvertExportUsesMetaDescriptionFallback(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "meta-description-fallback.json")

	result, err := deghosting.ConvertExport(export, "")
	require.NoError(t, err)
	require.Len(t, result.Posts, 1)
	require.Equal(t, "SEO fallback.", result.Posts[0].FrontMatter.Description)
	require.Empty(t, result.Posts[0].Assets)
}

func TestConvertExportSkipsPublishedPostWithoutPublishedAt(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "missing-published-at.json")

	result, err := deghosting.ConvertExport(export, "")
	require.NoError(t, err)
	require.Empty(t, result.Posts)
	require.Equal(t, []deghosting.Warning{
		{PostSlug: "missing-date", Message: "published post without published_at, skipping"},
	}, result.Warnings)
}

func TestConvertExportRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	export := parseExportFixture(t, "invalid-slug.json")

	result, err := deghosting.ConvertExport(export, "")
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
