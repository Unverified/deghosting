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

func TestProcessAssetsClassification(t *testing.T) {
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
			_, assets, err := processAssets(post, base)

			require.NoError(t, err)
			if tc.wantAsset {
				require.Len(t, assets, 1)
			} else {
				require.Empty(t, assets)
			}
		})
	}
}

func TestProcessAssetsResolvesRemoteURL(t *testing.T) {
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
			_, assets, err := processAssets(post, base)

			require.NoError(t, err)
			require.Len(t, assets, 1)
			require.Equal(t, tc.wantURL, assets[0].RemoteURL)
		})
	}
}

func TestProcessAssetsLocalPathDeterminism(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{Slug: "test", FeatureImage: "__GHOST_URL__/content/images/photo.jpg"}

	_, a, err := processAssets(post, base)
	require.NoError(t, err)
	_, b, err := processAssets(post, base)
	require.NoError(t, err)

	require.Equal(t, a[0].LocalPath, b[0].LocalPath)
	require.Contains(t, a[0].LocalPath, "photo.jpg")
}

func TestProcessAssetsLocalPathFormat(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{Slug: "test", FeatureImage: "__GHOST_URL__/content/images/photo.jpg"}
	_, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Regexp(t, `^[0-9a-f]{8}-photo\.jpg$`, assets[0].LocalPath)
}

func TestLocalPathFallsBackToAssetName(t *testing.T) {
	t.Parallel()

	got := localPath("https://blog.example.com")

	require.Regexp(t, `^[0-9a-f]{8}-asset$`, got)
}

func TestProcessAssetsDeduplication(t *testing.T) {
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

	_, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1, "same resolved URL should produce one asset")
	require.Equal(t, zola.AssetKindFeatureImage, assets[0].Kind, "first-seen kind wins")
}

func TestProcessAssetsKinds(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug:         "test",
		FeatureImage: "__GHOST_URL__/feature.png",
		OGImage:      "__GHOST_URL__/og.png",
		TwitterImage: "__GHOST_URL__/twitter.png",
		HTML: `<img src="__GHOST_URL__/body.png">
<audio src="__GHOST_URL__/audio.mp3">
	<source src="__GHOST_URL__/audio.ogg" type="audio/ogg">
	<track src="__GHOST_URL__/audio.vtt">
</audio>
<video src="__GHOST_URL__/video.mp4" poster="__GHOST_URL__/poster.jpg">
	<source src="__GHOST_URL__/video.webm" type="video/webm">
	<track src="__GHOST_URL__/video.vtt">
</video>`,
	}

	_, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 11)

	kinds := make([]zola.AssetKind, len(assets))
	for i, a := range assets {
		kinds[i] = a.Kind
	}
	require.Equal(t, []zola.AssetKind{
		zola.AssetKindFeatureImage,
		zola.AssetKindOpenGraphImage,
		zola.AssetKindTwitterImage,
		zola.AssetKindBodyImage,
		zola.AssetKindBodyAudio,
		zola.AssetKindBodyMediaSource,
		zola.AssetKindBodyMediaTrack,
		zola.AssetKindBodyVideo,
		zola.AssetKindBodyVideoPoster,
		zola.AssetKindBodyMediaSource,
		zola.AssetKindBodyMediaTrack,
	}, kinds)
}

func TestProcessAssetsExternalBodyImageSkipped(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<img src="https://external.com/photo.jpg"> <img src="__GHOST_URL__/local.jpg">`,
	}

	_, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0].RemoteURL, "local.jpg")
}

func TestProcessAssetsRewritesBodyHTML(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<p>Hello</p><img src="__GHOST_URL__/photo.jpg"><img src="https://external.com/ext.jpg">`,
	}

	rewritten, _, err := processAssets(post, base)

	require.NoError(t, err)
	require.NotContains(t, rewritten, "__GHOST_URL__")
	require.Regexp(t, `[0-9a-f]{8}-photo\.jpg`, rewritten)
	require.Contains(t, rewritten, "https://external.com/ext.jpg")
}

func TestProcessAssetsRewritesNativeMediaHTML(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<audio controls src="__GHOST_URL__/song.mp3">
	<source src="/song.ogg" type="audio/ogg">
	<track src="__GHOST_URL__/song.vtt">
</audio>
<video src="https://external.com/movie.mp4" poster="__GHOST_URL__/poster.jpg">
	<source src="__GHOST_URL__/movie.webm" type="video/webm">
</video>`,
	}

	rewritten, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 5)
	require.NotContains(t, rewritten, "__GHOST_URL__")
	require.Regexp(t, `[0-9a-f]{8}-song\.mp3`, rewritten)
	require.Regexp(t, `[0-9a-f]{8}-song\.ogg`, rewritten)
	require.Regexp(t, `[0-9a-f]{8}-song\.vtt`, rewritten)
	require.Regexp(t, `[0-9a-f]{8}-poster\.jpg`, rewritten)
	require.Regexp(t, `[0-9a-f]{8}-movie\.webm`, rewritten)
	require.Contains(t, rewritten, "https://external.com/movie.mp4")
}

func TestProcessAssetsIgnoresSrcsetAndPictureSource(t *testing.T) {
	t.Parallel()

	base := mustParseURL(t, testGhostURL)
	post := ghost.Post{
		Slug: "test",
		HTML: `<picture>
	<source src="__GHOST_URL__/ignored-source.webp" srcset="__GHOST_URL__/ignored-small.webp 400w">
	<img src="__GHOST_URL__/full.jpg" srcset="__GHOST_URL__/full-small.jpg 400w" alt="Full">
</picture>`,
	}

	rewritten, assets, err := processAssets(post, base)

	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0].RemoteURL, "full.jpg")
	require.Contains(t, rewritten, "__GHOST_URL__/ignored-source.webp")
	require.Contains(t, rewritten, "__GHOST_URL__/ignored-small.webp")
	require.Contains(t, rewritten, "__GHOST_URL__/full-small.jpg")
	require.Regexp(t, `[0-9a-f]{8}-full\.jpg`, rewritten)
}

func TestProcessAssetsGhostURLRequiredOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	t.Run("Ghost-hosted image ref without base errors", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{Slug: "my-post", FeatureImage: "__GHOST_URL__/photo.jpg"}
		_, _, err := processAssets(post, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "my-post")
		require.Contains(t, err.Error(), "--ghost-url")
	})

	t.Run("Ghost-hosted media ref without base errors", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{
			Slug: "my-post",
			HTML: `<video src="__GHOST_URL__/movie.mp4" poster="__GHOST_URL__/poster.jpg"></video>`,
		}
		_, _, err := processAssets(post, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "my-post")
		require.Contains(t, err.Error(), "--ghost-url")
	})

	t.Run("only external refs without base is fine", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{
			Slug: "my-post",
			HTML: `<img src="https://external.com/photo.jpg"><audio src="https://cdn.external.com/song.mp3"></audio>`,
		}
		_, assets, err := processAssets(post, nil)

		require.NoError(t, err)
		require.Empty(t, assets)
	})

	t.Run("no refs at all without base is fine", func(t *testing.T) {
		t.Parallel()

		post := ghost.Post{Slug: "my-post"}
		_, assets, err := processAssets(post, nil)

		require.NoError(t, err)
		require.Empty(t, assets)
	})
}

func TestConvertBodyHTMLPreservesNativeMedia(t *testing.T) {
	t.Parallel()

	input := `<figure class="kg-card kg-video-card">
	<video controls poster="poster.jpg">
		<source src="movie.mp4" type="video/mp4">
	</video>
	<figcaption>Demo caption</figcaption>
</figure>`

	got, err := convertBodyHTML(input)

	require.NoError(t, err)
	require.Contains(t, got, `<figure class="kg-card kg-video-card">`)
	require.Contains(t, got, `<video controls="" poster="poster.jpg">`)
	require.Contains(t, got, `src="movie.mp4"`)
	require.Contains(t, got, `<figcaption>Demo caption</figcaption>`)
}

func TestConvertBodyHTMLPreservesExternalNativeMedia(t *testing.T) {
	t.Parallel()

	got, err := convertBodyHTML(`<p>Listen</p><audio controls src="https://cdn.example.com/song.mp3"></audio>`)

	require.NoError(t, err)
	require.Contains(t, got, "Listen")
	require.Contains(t, got, `<audio controls="" src="https://cdn.example.com/song.mp3"></audio>`)
}

func TestConvertBodyHTMLKeepsOrdinaryImageFigureAsMarkdown(t *testing.T) {
	t.Parallel()

	got, err := convertBodyHTML(`<figure><img src="photo.jpg" alt="Alt"><figcaption>Caption</figcaption></figure>`)

	require.NoError(t, err)
	require.Contains(t, got, `![Alt](photo.jpg)`)
	require.Contains(t, got, `Caption`)
	require.NotContains(t, got, `<figure`)
}

func TestConvertBodyHTMLDropsResponsiveImageSources(t *testing.T) {
	t.Parallel()

	input := `<picture>
	<source srcset="small.webp 400w, large.webp 800w" type="image/webp">
	<img src="full.jpg" srcset="small.jpg 400w, full.jpg 800w" alt="Full">
</picture>`

	got, err := convertBodyHTML(input)

	require.NoError(t, err)
	require.Equal(t, `![Full](full.jpg)`, got)
}
