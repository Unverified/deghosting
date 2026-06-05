package deghosting

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Unverified/deghosting/internal/download"
	"github.com/Unverified/deghosting/internal/zola"
)

// Operation runs the full deghosting workflow and owns user-facing output.
type Operation struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Downloader download.AssetDownloader
	convert    convertFunc
	writePosts writePostsFunc
}

type (
	convertFunc    func(io.Reader, string) (ConvertResult, error)
	writePostsFunc func(string, []zola.Post) error
)

// Process converts the Ghost export, downloads Ghost-hosted assets, writes
// generated files, and reports warnings and a summary.
func (o Operation) Process(export io.Reader, out string, ghostURL string) error {
	stdout := o.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	stderr := o.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	convert := o.convert
	if convert == nil {
		convert = Convert
	}

	write := o.writePosts
	if write == nil {
		write = writePosts
	}

	downloader := o.Downloader
	if downloader == nil {
		return errors.New("asset downloader is nil")
	}

	result, err := convert(export, ghostURL)
	if err != nil {
		downloader.Close()
		return err
	}

	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintln(stderr, "warning:", warning)
	}
	for _, skipped := range result.Skipped {
		_, _ = fmt.Fprintln(stderr, "skipped:", skipped)
	}
	_, _ = fmt.Fprintf(stdout,
		"conversion finished, %d posts converted, %d posts skipped, %d warnings\n",
		len(result.Posts),
		len(result.Skipped),
		len(result.Warnings),
	)

	_, _ = fmt.Fprintf(stdout, "starting to write posts to %s\n", out)
	if err = write(out, result.Posts); err != nil {
		downloader.Close()
		return err
	}

	totalAssets := 0
	slugsByDestPath := make(map[string]string)
	for _, post := range result.Posts {
		totalAssets += len(post.Assets)
		for _, asset := range post.Assets {
			destPath := filepath.Join(out, post.Slug, asset.LocalPath)
			slugsByDestPath[destPath] = post.Slug
			downloader.Submit(asset.RemoteURL, destPath)
		}
	}

	downloadFailures := downloader.Close()
	for _, failure := range downloadFailures {
		warning := Warning{
			PostSlug: slugsByDestPath[failure.DestPath],
			Message:  fmt.Sprintf("failed to download %s: %v", failure.RemoteURL, failure.Err),
		}
		_, _ = fmt.Fprintln(stderr, "warning:", warning)
	}

	if totalAssets > 0 {
		nFailures := len(downloadFailures)
		_, _ = fmt.Fprintf(stdout, "%d assets downloaded, %d failed\n", totalAssets-nFailures, nFailures)
	}

	_, _ = fmt.Fprintln(stdout, "done!")
	return nil
}
