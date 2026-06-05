package deghosting

import (
	"fmt"
	"io"
	"net/http"

	"github.com/Unverified/deghosting/internal/zola"
)

// Operation runs the full deghosting workflow and owns user-facing output.
type Operation struct {
	Stdout io.Writer
	Stderr io.Writer
	// Doer is the HTTP client used for image downloads. Defaults to
	// http.DefaultClient when nil.
	Doer       Doer
	convert    convertFunc
	writePosts writePostsFunc
	download   downloadFunc
}

type (
	convertFunc    func(io.Reader, string) (ConvertResult, error)
	writePostsFunc func(string, []zola.Post) error
)

// Process converts the Ghost export, downloads Ghost-hosted images, writes
// generated files, and reports warnings and a summary.
func (o Operation) Process(export io.Reader, out string, ghostURL string, cfg DownloadConfig) error {
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
	download := o.download
	if download == nil {
		download = downloadAssets
	}
	doer := o.Doer
	if doer == nil {
		doer = http.DefaultClient
	}

	result, err := convert(export, ghostURL)
	if err != nil {
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
		return err
	}

	downloadWarnings := download(out, result.Posts, doer, cfg)
	for _, w := range downloadWarnings {
		_, _ = fmt.Fprintln(stderr, "warning:", w)
	}

	totalAssets := 0
	for _, p := range result.Posts {
		totalAssets += len(p.Assets)
	}
	if totalAssets > 0 {
		_, _ = fmt.Fprintf(stdout, "%d assets downloaded, %d failed\n", totalAssets-len(downloadWarnings), len(downloadWarnings))
	}

	_, _ = fmt.Fprintln(stdout, "done!")
	return nil
}
