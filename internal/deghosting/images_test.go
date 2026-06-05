package deghosting

import (
	"net/url"
	"testing"

	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

const testGhostURL = "https://blog.example.com"

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	require.NoError(t, err)
	return u
}

func TestProcessImagesClassification(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	cases := []struct {
		name         string
		featureImage string
		wantAsset    bool
	}{
		{
			name:         "placeholder",
			featureImage: "__GHOST_URL__/content/images/photo.jpg",
			wantAsset:    true,
		},
		{
			name:         "site-relative path",
			featureImage: "/content/images/photo.jpg",
			wantAsset:    true,
		},
		{
			name:         "absolute Ghost-origin URL",
			featureImage: "https://blog.example.com/content/images/photo.jpg",
			wantAsset:    true,
		},
		{
			name:         "same host different scheme",
			featureImage: "http://blog.example.com/content/images/photo.jpg",
			wantAsset:    false,
		},
		{
			name:         "external host",
			featureImage: "https://cdn.other.com/photo.jpg",
			wantAsset:    false,
		},
		{
			name:         "protocol-relative external host",
			featureImage: "//cdn.other.com/photo.jpg",
			wantAsset:    false,
		},
		{
			name:         "external CDN-style host",
			featureImage: "https://images.unsplash.com/photo.jpg",
			wantAsset:    false,
		},
		{
			name:         "empty source",
			featureImage: "",
			wantAsset:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			post := ghost.Post{Slug: "test", FeatureImage: tc.featureImage}
			_, assets, err := processImages(post, base)

			require.NoError(t, err)
			if tc.wantAsset {
				require.Len(t, assets, 1)
			} else {
				require.Empty(t, assets)
			}
		})
	}
}

func TestProcessImagesResolvesRemoteURL(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	cases := []struct {
		name    string
		src     string
		wantURL string
	}{
		{
			name:    "placeholder",
			src:     "__GHOST_URL__/content/images/photo.jpg",
			wantURL: "https://blog.example.com/content/images/photo.jpg",
		},
		{
			name:    "site-relative",
			src:     "/content/images/photo.jpg",
			wantURL: "https://blog.example.com/content/images/photo.jpg",
		},
		{
			name:    "absolute Ghost-origin",
			src:     "https://blog.example.com/content/images/photo.jpg",
			wantURL: "https://blog.example.com/content/images/photo.jpg",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			post := ghost.Post{Slug: "test", FeatureImage: tc.src}
			_, assets, err := processImages(post, base)

			require.NoError(t, err)
			require.Len(t, assets, 1)
			require.Equal(t, tc.wantURL, assets[0].RemoteURL)
		})
	}
}

func TestProcessImagesLocalPathDeterminism(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{Slug: "test", FeatureImage: "__GHOST_URL__/content/images/photo.jpg"}

	_, a, err := processImages(post, base)
	require.NoError(t, err)
	_, b, err := processImages(post, base)
	require.NoError(t, err)

	require.Equal(t, a[0].LocalPath, b[0].LocalPath)
	require.Contains(t, a[0].LocalPath, "photo.jpg")
}

func TestProcessImagesLocalPathFormat(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{Slug: "test", FeatureImage: "__GHOST_URL__/content/images/photo.jpg"}
	_, assets, err := processImages(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Regexp(t, `^[0-9a-f]{8}-photo\.jpg$`, assets[0].LocalPath)
}

func TestProcessImagesDeduplication(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	src := "__GHOST_URL__/content/images/shared.jpg"
	post := ghost.Post{
		Slug:         "test",
		FeatureImage: src,
		OGImage:      src,
		TwitterImage: src,
		HTML:         `<img src="` + src + `">`,
	}

	_, assets, err := processImages(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1, "same resolved URL should produce one asset")
	require.Equal(t, zola.ImageKindFeature, assets[0].Kind, "first-seen kind wins")
}

func TestProcessImagesKinds(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug:         "test",
		FeatureImage: "__GHOST_URL__/feature.png",
		OGImage:      "__GHOST_URL__/og.png",
		TwitterImage: "__GHOST_URL__/twitter.png",
		HTML:         `<img src="__GHOST_URL__/body.png">`,
	}

	_, assets, err := processImages(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 4)

	kinds := make([]zola.ImageKind, len(assets))
	for i, a := range assets {
		kinds[i] = a.Kind
	}
	require.Equal(t, []zola.ImageKind{
		zola.ImageKindFeature,
		zola.ImageKindOpenGraph,
		zola.ImageKindTwitter,
		zola.ImageKindBody,
	}, kinds)
}

func TestProcessImagesExternalBodyImageSkipped(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<img src="https://external.com/photo.jpg"> <img src="__GHOST_URL__/local.jpg">`,
	}

	_, assets, err := processImages(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0].RemoteURL, "local.jpg")
}

func TestProcessImagesRewritesBodyHTML(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<p>Hello</p><img src="__GHOST_URL__/photo.jpg"><img src="https://external.com/ext.jpg">`,
	}

	rewritten, _, err := processImages(post, base)

	require.NoError(t, err)
	require.NotContains(t, rewritten, "__GHOST_URL__")
	require.Regexp(t, `[0-9a-f]{8}-photo\.jpg`, rewritten)
	require.Contains(t, rewritten, "https://external.com/ext.jpg")
}

func TestProcessImagesGhostURLRequiredOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	t.Run("Ghost-hosted ref without base errors", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{Slug: "my-post", FeatureImage: "__GHOST_URL__/photo.jpg"}
		_, _, err := processImages(post, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "my-post")
		require.Contains(t, err.Error(), "--ghost-url")
	})

	t.Run("only external refs without base is fine", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{
			Slug: "my-post",
			HTML: `<img src="https://external.com/photo.jpg"><img src="//cdn.external.com/photo.jpg">`,
		}
		_, assets, err := processImages(post, nil)

		require.NoError(t, err)
		require.Empty(t, assets)
	})

	t.Run("no refs at all without base is fine", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{Slug: "my-post"}
		_, assets, err := processImages(post, nil)

		require.NoError(t, err)
		require.Empty(t, assets)
	})
}
