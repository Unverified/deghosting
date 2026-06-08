package deghosting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteBodyLinksRewritesConvertedPostLinks(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	convertedSlugs := map[string]struct{}{
		"hello-world": {},
		"other-post":  {},
	}
	input := `<p>
	<a href="__GHOST_URL__/hello-world/?utm_source=ghost#comments"><strong>Hello</strong></a>
	<a href="/other-post">Other</a>
	<a href="https://blog.example.com/hello-world/">Absolute</a>
	<a href="https://external.com/hello-world/">External</a>
</p>`

	got, warnings := rewriteBodyLinks("source-post", input, base, convertedSlugs)

	require.Empty(t, warnings)
	require.Contains(t, got, `href="@/hello-world/index.md#comments"`)
	require.Contains(t, got, `href="@/other-post/index.md"`)
	require.Contains(t, got, `href="@/hello-world/index.md"`)
	require.Contains(t, got, `href="https://external.com/hello-world/"`)
	require.NotContains(t, got, "utm_source")
}

func TestRewriteBodyLinksDoesNotRequireGhostURLForPlaceholderOrRelativeLinks(t *testing.T) {
	t.Parallel()

	convertedSlugs := map[string]struct{}{
		"hello-world": {},
	}
	input := `<p>
	<a href="__GHOST_URL__/hello-world/">Placeholder</a>
	<a href="/hello-world/">Relative</a>
	<a href="https://blog.example.com/hello-world/">Absolute</a>
</p>`

	got, warnings := rewriteBodyLinks("source-post", input, nil, convertedSlugs)

	require.Empty(t, warnings)
	require.Contains(t, got, `<a href="@/hello-world/index.md">Placeholder</a>`)
	require.Contains(t, got, `<a href="@/hello-world/index.md">Relative</a>`)
	require.Contains(t, got, `<a href="https://blog.example.com/hello-world/">Absolute</a>`)
}

func TestRewriteBodyLinksWarnsOnceForUnmatchedGhostHostedLinks(t *testing.T) {
	t.Parallel()

	input := `<p>
	<a href="__GHOST_URL__/missing-post/">Missing</a>
	<a href="__GHOST_URL__/missing-post/">Missing again</a>
	<a href="/tag/news/">Tag</a>
</p>`

	got, warnings := rewriteBodyLinks("source-post", input, nil, map[string]struct{}{})

	require.Contains(t, got, `href="__GHOST_URL__/missing-post/"`)
	require.Contains(t, got, `href="/tag/news/"`)
	require.Equal(t, []Warning{
		{
			PostSlug: "source-post",
			Message:  "Ghost-hosted body link __GHOST_URL__/missing-post/ does not match a converted post",
		},
		{
			PostSlug: "source-post",
			Message:  "Ghost-hosted body link /tag/news/ does not match a converted post",
		},
	}, warnings)
}

func TestRewriteBodyLinksDecodesPathBeforeMatching(t *testing.T) {
	t.Parallel()

	convertedSlugs := map[string]struct{}{
		"hello-world": {},
	}

	got, warnings := rewriteBodyLinks(
		"source-post",
		`<a href="__GHOST_URL__/hello%2Dworld/">Hello</a>`,
		nil,
		convertedSlugs,
	)

	require.Empty(t, warnings)
	require.Contains(t, got, `href="@/hello-world/index.md"`)
}

func TestRewriteBodyLinksRewritesLinksInsideMediaFigures(t *testing.T) {
	t.Parallel()

	convertedSlugs := map[string]struct{}{
		"hello-world": {},
	}
	input := `<figure>
	<video src="movie.mp4"></video>
	<figcaption><a href="__GHOST_URL__/hello-world/">More</a></figcaption>
</figure>`

	got, warnings := rewriteBodyLinks("source-post", input, nil, convertedSlugs)

	require.Empty(t, warnings)
	require.Contains(t, got, `<a href="@/hello-world/index.md">More</a>`)
}
