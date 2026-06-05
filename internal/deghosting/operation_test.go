package deghosting

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Unverified/deghosting/internal/download"
	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

// stubDownloader is an in-memory download.AssetDownloader for operation tests.
type stubDownloader struct {
	submitted []submittedDownload
	failures  []download.Failure
	closed    bool
}

type submittedDownload struct {
	remoteURL string
	destPath  string
}

func (s *stubDownloader) Submit(remoteURL, destPath string) {
	s.submitted = append(s.submitted, submittedDownload{
		remoteURL: remoteURL,
		destPath:  destPath,
	})
}

func (s *stubDownloader) Close() []download.Failure {
	s.closed = true
	return s.failures
}

func TestOperationProcessReportsWarningsSkippedAndSummary(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("ghost export")
	out := "content"
	posts := []zola.Post{
		{
			Slug: "hello-world",
			FrontMatter: zola.FrontMatter{
				Title: "Hello World",
				Date:  time.Date(2020, 5, 19, 12, 3, 0, 0, time.UTC),
			},
		},
	}
	result := ConvertResult{
		Posts: posts,
		Skipped: []SkippedPost{
			{Slug: "draft", Reason: "not published"},
		},
		Warnings: []Warning{
			{PostSlug: "missing-date", Message: "published post without published_at, skipping"},
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var downloader stubDownloader

	err := Operation{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Downloader: &downloader,
		convert: func(got io.Reader, _ string) (ConvertResult, error) {
			require.Same(t, input, got)
			return result, nil
		},
		writePosts: func(gotOut string, gotPosts []zola.Post) error {
			require.Equal(t, out, gotOut)
			require.Equal(t, posts, gotPosts)
			return nil
		},
	}.Process(input, out, "")

	require.NoError(t, err)
	require.True(t, downloader.closed)
	require.Contains(t, stdout.String(), "1 posts converted")
	require.Contains(t, stdout.String(), "1 posts skipped")
	require.Contains(t, stdout.String(), "1 warnings")
	require.Contains(t, stdout.String(), "starting to write posts to content")
	require.Contains(t, stdout.String(), "done!")
	require.Equal(t, "warning: missing-date: published post without published_at, skipping\nskipped: draft: not published\n", stderr.String())
}

func TestOperationProcessReportsDownloadFailures(t *testing.T) {
	t.Parallel()

	assetURL := "https://example.com/img.jpg"
	post := zola.Post{
		Slug:   "hello-world",
		Assets: []zola.ImageAsset{{RemoteURL: assetURL, LocalPath: "abc-img.jpg"}},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	failure := download.Failure{
		RemoteURL: assetURL,
		DestPath:  filepath.Join("content", "hello-world", "abc-img.jpg"),
	}

	err := Operation{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Downloader: &stubDownloader{failures: []download.Failure{failure}},
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: []zola.Post{post}}, nil
		},
		writePosts: func(string, []zola.Post) error { return nil },
	}.Process(strings.NewReader("ghost export"), "content", "")

	require.NoError(t, err)
	require.Contains(t, stderr.String(), "hello-world")
	require.Contains(t, stderr.String(), assetURL)
	require.Contains(t, stdout.String(), "0 assets downloaded, 1 failed")
}

func TestOperationProcessSubmitsAssetsToDownloader(t *testing.T) {
	t.Parallel()

	assetURL := "https://example.com/img.jpg"
	expectedSubmission := submittedDownload{
		remoteURL: assetURL,
		destPath:  filepath.Join("content", "hello-world", "abc-img.jpg"),
	}
	post := zola.Post{
		Slug:   "hello-world",
		Assets: []zola.ImageAsset{{RemoteURL: assetURL, LocalPath: "abc-img.jpg"}},
	}

	var stub stubDownloader
	err := Operation{
		Downloader: &stub,
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: []zola.Post{post}}, nil
		},
		writePosts: func(string, []zola.Post) error { return nil },
	}.Process(strings.NewReader("ghost export"), "content", "")

	require.NoError(t, err)
	require.True(t, stub.closed)
	require.Equal(t, []submittedDownload{expectedSubmission}, stub.submitted)
}

func TestOperationProcessReportsFailedDuplicateURLWithOwningPost(t *testing.T) {
	t.Parallel()

	assetURL := "https://example.com/img.jpg"
	posts := []zola.Post{
		{
			Slug:   "first-post",
			Assets: []zola.ImageAsset{{RemoteURL: assetURL, LocalPath: "first-img.jpg"}},
		},
		{
			Slug:   "second-post",
			Assets: []zola.ImageAsset{{RemoteURL: assetURL, LocalPath: "second-img.jpg"}},
		},
	}
	failure := download.Failure{
		RemoteURL: assetURL,
		DestPath:  filepath.Join("content", "first-post", "first-img.jpg"),
	}

	var stderr bytes.Buffer
	err := Operation{
		Stderr:     &stderr,
		Downloader: &stubDownloader{failures: []download.Failure{failure}},
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: posts}, nil
		},
		writePosts: func(string, []zola.Post) error { return nil },
	}.Process(strings.NewReader("ghost export"), "content", "")

	require.NoError(t, err)
	require.Contains(t, stderr.String(), "warning: first-post: failed to download")
	require.NotContains(t, stderr.String(), "second-post")
}

func TestOperationProcessReturnsConvertError(t *testing.T) {
	t.Parallel()

	convertErr := errors.New("convert failed")
	writeCalled := false
	var downloader stubDownloader

	err := Operation{
		Downloader: &downloader,
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{}, convertErr
		},
		writePosts: func(string, []zola.Post) error {
			writeCalled = true
			return nil
		},
	}.Process(strings.NewReader("ghost export"), "content", "")

	require.ErrorIs(t, err, convertErr)
	require.False(t, writeCalled)
	require.True(t, downloader.closed)
}

func TestOperationProcessReturnsWriteError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	post := zola.Post{Slug: "hello-world"}
	var downloader stubDownloader

	err := Operation{
		Downloader: &downloader,
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: []zola.Post{post}}, nil
		},
		writePosts: func(out string, posts []zola.Post) error {
			require.Equal(t, "content", out)
			require.Equal(t, []zola.Post{post}, posts)
			return writeErr
		},
	}.Process(strings.NewReader("ghost export"), "content", "")

	require.ErrorIs(t, err, writeErr)
	require.True(t, downloader.closed)
}
