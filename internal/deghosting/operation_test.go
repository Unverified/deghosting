package deghosting

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

var noCfg = DownloadConfig{Timeout: time.Second, Concurrency: 1}

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

	err := Operation{
		Stdout: &stdout,
		Stderr: &stderr,
		convert: func(got io.Reader, _ string) (ConvertResult, error) {
			require.Same(t, input, got)
			return result, nil
		},
		writePosts: func(gotOut string, gotPosts []zola.Post) error {
			require.Equal(t, out, gotOut)
			require.Equal(t, posts, gotPosts)
			return nil
		},
		download: func(string, []zola.Post, Doer, DownloadConfig) []Warning { return nil },
	}.Process(input, out, "", noCfg)

	require.NoError(t, err)
	require.Contains(t, stdout.String(), "1 posts converted")
	require.Contains(t, stdout.String(), "1 posts skipped")
	require.Contains(t, stdout.String(), "1 warnings")
	require.Contains(t, stdout.String(), "starting to write posts to content")
	require.Contains(t, stdout.String(), "done!")
	require.Equal(t, "warning: missing-date: published post without published_at, skipping\nskipped: draft: not published\n", stderr.String())
}

func TestOperationProcessReportsDownloadWarnings(t *testing.T) {
	t.Parallel()

	post := zola.Post{
		Slug:   "hello-world",
		Assets: []zola.ImageAsset{{RemoteURL: "https://example.com/img.jpg", LocalPath: "abc-img.jpg"}},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Operation{
		Stdout: &stdout,
		Stderr: &stderr,
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: []zola.Post{post}}, nil
		},
		writePosts: func(string, []zola.Post) error { return nil },
		download: func(string, []zola.Post, Doer, DownloadConfig) []Warning {
			return []Warning{{PostSlug: "hello-world", Message: "failed to download https://example.com/img.jpg after 3 attempts"}}
		},
	}.Process(strings.NewReader("ghost export"), "content", "", noCfg)

	require.NoError(t, err)
	require.Contains(t, stderr.String(), "warning: hello-world: failed to download")
	require.Contains(t, stdout.String(), "0 assets downloaded, 1 failed")
}

func TestOperationProcessReturnsConvertError(t *testing.T) {
	t.Parallel()

	convertErr := errors.New("convert failed")
	writeCalled := false

	err := Operation{
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{}, convertErr
		},
		writePosts: func(string, []zola.Post) error {
			writeCalled = true
			return nil
		},
	}.Process(strings.NewReader("ghost export"), "content", "", noCfg)

	require.ErrorIs(t, err, convertErr)
	require.False(t, writeCalled)
}

func TestOperationProcessReturnsWriteError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	post := zola.Post{Slug: "hello-world"}

	err := Operation{
		convert: func(io.Reader, string) (ConvertResult, error) {
			return ConvertResult{Posts: []zola.Post{post}}, nil
		},
		writePosts: func(out string, posts []zola.Post) error {
			require.Equal(t, "content", out)
			require.Equal(t, []zola.Post{post}, posts)
			return writeErr
		},
	}.Process(strings.NewReader("ghost export"), "content", "", noCfg)

	require.ErrorIs(t, err, writeErr)
}
