package deghosting

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

const testTimeout = 5 * time.Second

// stubDoer is an injectable HTTP client for tests.
type stubDoer struct {
	mu       sync.Mutex
	attempts map[string]int
	failFor  map[string]int // fail this many times before succeeding
	body     []byte
}

func newStubDoer(body []byte) *stubDoer {
	return &stubDoer{
		attempts: make(map[string]int),
		failFor:  make(map[string]int),
		body:     body,
	}
}

func (s *stubDoer) failURL(url string, times int) {
	s.mu.Lock()
	s.failFor[url] = times
	s.mu.Unlock()
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	key := req.URL.String()
	s.attempts[key]++
	att := s.attempts[key]
	fails := s.failFor[key]
	s.mu.Unlock()

	if att <= fails {
		return nil, fmt.Errorf("stub failure attempt %d", att)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     http.Header{},
	}, nil
}

func (s *stubDoer) callCount(url string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts[url]
}

func TestDownloadAssetsSuccess(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := []byte("image-bytes")
	doer := newStubDoer(content)

	posts := []zola.Post{{
		Slug: "my-post",
		Assets: []zola.ImageAsset{{
			RemoteURL: "https://example.com/photo.jpg",
			LocalPath: "abc123-photo.jpg",
		}},
	}}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 1})

	require.Empty(t, warnings)
	got, err := os.ReadFile(filepath.Join(root, "my-post", "abc123-photo.jpg"))
	require.NoError(t, err)
	require.Equal(t, content, got)
}

func TestDownloadAssetsRetrySucceeds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	doer := newStubDoer([]byte("img"))
	url := "https://example.com/photo.jpg"
	doer.failURL(url, 2) // fail twice, succeed on third attempt

	posts := []zola.Post{{
		Slug:   "my-post",
		Assets: []zola.ImageAsset{{RemoteURL: url, LocalPath: "abc-photo.jpg"}},
	}}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 1})

	require.Empty(t, warnings)
	require.Equal(t, 3, doer.callCount(url))
	_, err := os.Stat(filepath.Join(root, "my-post", "abc-photo.jpg"))
	require.NoError(t, err)
}

func TestDownloadAssetsAllRetriesFail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	doer := newStubDoer(nil)
	url := "https://example.com/photo.jpg"
	doer.failURL(url, 99) // always fail

	posts := []zola.Post{{
		Slug:   "my-post",
		Assets: []zola.ImageAsset{{RemoteURL: url, LocalPath: "abc-photo.jpg"}},
	}}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 1})

	require.Len(t, warnings, 1)
	require.Equal(t, "my-post", warnings[0].PostSlug)
	require.Contains(t, warnings[0].Message, url)
	require.Equal(t, 3, doer.callCount(url))
}

func TestDownloadAssetsContinuesAfterFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	doer := newStubDoer([]byte("img"))

	failURL := "https://example.com/bad.jpg"
	goodURL := "https://example.com/good.jpg"
	doer.failURL(failURL, 99)

	posts := []zola.Post{{
		Slug: "my-post",
		Assets: []zola.ImageAsset{
			{RemoteURL: failURL, LocalPath: "bad.jpg"},
			{RemoteURL: goodURL, LocalPath: "good.jpg"},
		},
	}}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 2})

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0].Message, failURL)
	_, err := os.Stat(filepath.Join(root, "my-post", "good.jpg"))
	require.NoError(t, err, "successful asset should still be written")
}

func TestDownloadAssetsOverwritesExisting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundleDir := filepath.Join(root, "my-post")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	target := filepath.Join(bundleDir, "abc-photo.jpg")
	require.NoError(t, os.WriteFile(target, []byte("old-content"), 0o644))

	newContent := []byte("new-content")
	doer := newStubDoer(newContent)
	posts := []zola.Post{{
		Slug:   "my-post",
		Assets: []zola.ImageAsset{{RemoteURL: "https://example.com/photo.jpg", LocalPath: "abc-photo.jpg"}},
	}}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 1})

	require.Empty(t, warnings)
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, newContent, got, "file should be overwritten")
}

func TestDownloadAssetsNoPosts(t *testing.T) {
	t.Parallel()

	warnings := downloadAssets(t.TempDir(), nil, newStubDoer(nil), DownloadConfig{Timeout: testTimeout, Concurrency: 1})
	require.Empty(t, warnings)
}

func TestDownloadAssetsConcurrency(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var mu sync.Mutex

	doer := &callbackDoer{fn: func(req *http.Request) (*http.Response, error) {
		cur := inFlight.Add(1)
		mu.Lock()
		if cur > maxInFlight.Load() {
			maxInFlight.Store(cur)
		}
		mu.Unlock()
		// small delay so workers actually overlap
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("img"))),
			Header:     http.Header{},
		}, nil
	}}

	// Build 10 assets across 2 posts.
	var posts []zola.Post
	for i := range 2 {
		var assets []zola.ImageAsset
		for j := range 5 {
			assets = append(assets, zola.ImageAsset{
				RemoteURL: fmt.Sprintf("https://example.com/p%d/img%d.jpg", i, j),
				LocalPath: fmt.Sprintf("img%d.jpg", j),
			})
		}
		posts = append(posts, zola.Post{Slug: fmt.Sprintf("post-%d", i), Assets: assets})
	}

	warnings := downloadAssets(root, posts, doer, DownloadConfig{Timeout: testTimeout, Concurrency: 4})

	require.Empty(t, warnings)
	require.LessOrEqual(t, maxInFlight.Load(), int32(4), "concurrency must not exceed limit")
}

// callbackDoer lets tests inject custom Do behaviour.
type callbackDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (c *callbackDoer) Do(req *http.Request) (*http.Response, error) {
	return c.fn(req)
}
