package zola_test

import (
	"testing"

	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

func TestPostPaths(t *testing.T) {
	t.Parallel()

	post := zola.Post{Slug: "announcing-usdai-token-sale"}

	require.Equal(t, "announcing-usdai-token-sale", post.BundleDir())
	require.Equal(t, "announcing-usdai-token-sale/index.md", post.MarkdownPath())
}
