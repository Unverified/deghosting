package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func newStartedDownloader(ctx context.Context, doer HTTPDoer) *Downloader {
	return New(ctx, doer, Config{Concurrency: 1})
}

func TestDownloaderSuccess(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := []byte("asset-bytes")
	doer := newStubDoer(content)

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit("https://example.com/photo.jpg", filepath.Join(root, "abc123-photo.jpg"))
	failures := dl.Close()

	require.Empty(t, failures)
	got, err := os.ReadFile(filepath.Join(root, "abc123-photo.jpg"))
	require.NoError(t, err)
	require.Equal(t, content, got)
}

func TestDownloaderRetrySucceeds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	url := "https://example.com/photo.jpg"
	var calls atomic.Int32
	doer := newRetryableTestDoer(func(_ *http.Request) (*http.Response, error) {
		if calls.Add(1) <= 2 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Header:     http.Header{},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("asset"))),
			Header:     http.Header{},
		}, nil
	})

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit(url, filepath.Join(root, "abc-photo.jpg"))
	failures := dl.Close()

	require.Empty(t, failures)
	require.Equal(t, int32(3), calls.Load())
}

func TestDownloaderAllRetriesFail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	url := "https://example.com/photo.jpg"
	destPath := filepath.Join(root, "abc-photo.jpg")
	var calls atomic.Int32
	doer := newRetryableTestDoer(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     http.Header{},
		}, nil
	})

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit(url, destPath)
	failures := dl.Close()

	require.Len(t, failures, 1)
	require.Equal(t, url, failures[0].RemoteURL)
	require.Equal(t, destPath, failures[0].DestPath)
	require.Error(t, failures[0].Err)
	require.Equal(t, int32(3), calls.Load())
}

func TestDownloaderDoesNotRetryNonRetryableStatus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	url := "https://example.com/not-found.jpg"
	var calls atomic.Int32
	doer := newRetryableTestDoer(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     http.Header{},
		}, nil
	})

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit(url, filepath.Join(root, "not-found.jpg"))
	failures := dl.Close()

	require.Len(t, failures, 1)
	require.Equal(t, url, failures[0].RemoteURL)
	require.Equal(t, int32(1), calls.Load())
}

func TestDownloaderDoesNotRetryInstallFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	url := "https://example.com/photo.jpg"
	destPath := filepath.Join(root, "existing-dir")
	require.NoError(t, os.Mkdir(destPath, 0o755))

	var calls atomic.Int32
	doer := newRetryableTestDoer(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("asset"))),
			Header:     http.Header{},
		}, nil
	})

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit(url, destPath)
	failures := dl.Close()

	require.Len(t, failures, 1)
	require.Equal(t, url, failures[0].RemoteURL)
	require.Equal(t, destPath, failures[0].DestPath)
	require.Equal(t, int32(1), calls.Load())
}

func TestDownloaderContinuesAfterFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	doer := newStubDoer([]byte("asset"))
	failURL := "https://example.com/bad.jpg"
	goodURL := "https://example.com/good.jpg"
	doer.failURL(failURL, 99)

	failDestPath := filepath.Join(root, "bad.jpg")
	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit(failURL, failDestPath)
	dl.Submit(goodURL, filepath.Join(root, "good.jpg"))
	failures := dl.Close()

	require.Len(t, failures, 1)
	require.Equal(t, failURL, failures[0].RemoteURL)
	require.Equal(t, failDestPath, failures[0].DestPath)
	require.Error(t, failures[0].Err)
	_, err := os.Stat(filepath.Join(root, "good.jpg"))
	require.NoError(t, err, "successful asset should still be written")
}

func TestDownloaderOverwritesExisting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "photo.jpg")
	require.NoError(t, os.WriteFile(target, []byte("old-content"), 0o644))

	newContent := []byte("new-content")
	doer := newStubDoer(newContent)

	dl := newStartedDownloader(t.Context(), doer)
	dl.Submit("https://example.com/photo.jpg", target)
	failures := dl.Close()

	require.Empty(t, failures)
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, newContent, got)
}

func TestDownloaderNoJobs(t *testing.T) {
	t.Parallel()

	dl := newStartedDownloader(t.Context(), newStubDoer(nil))
	require.Empty(t, dl.Close())
}

func TestDownloaderConcurrencyCap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var mu sync.Mutex

	doer := &callbackDoer{fn: func(_ *http.Request) (*http.Response, error) {
		cur := inFlight.Add(1)
		mu.Lock()
		if cur > maxInFlight.Load() {
			maxInFlight.Store(cur)
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("asset"))),
			Header:     http.Header{},
		}, nil
	}}

	dl := newStartedDownloader(t.Context(), doer)
	for i := range 9 {
		dl.Submit(
			fmt.Sprintf("https://example.com/img%d.jpg", i),
			filepath.Join(root, fmt.Sprintf("img%d.jpg", i)),
		)
	}

	require.Empty(t, dl.Close())
	require.LessOrEqual(t, maxInFlight.Load(), int32(3))
}

type callbackDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (c *callbackDoer) Do(req *http.Request) (*http.Response, error) {
	return c.fn(req)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newRetryableTestDoer(fn func(*http.Request) (*http.Response, error)) HTTPDoer {
	base := &http.Client{
		Transport: roundTripFunc(fn),
	}
	return NewRetryableHTTPClient(base)
}
