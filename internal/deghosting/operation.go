package deghosting

import (
	"fmt"
	"io"

	"github.com/Unverified/deghosting/internal/zola"
)

// Operation runs the full deghosting workflow and owns user-facing output.
type Operation struct {
	Stdout     io.Writer
	Stderr     io.Writer
	convert    convertFunc
	writePosts writePostsFunc
}

type convertFunc func(io.Reader) (ConvertResult, error)
type writePostsFunc func(string, []zola.Post) error

// Process converts the Ghost export, writes generated files, and reports
// warnings and a summary.
func (o Operation) Process(export io.Reader, out string) error {
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

	result, err := convert(export)
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
	_, _ = fmt.Fprintln(stdout, "done!")

	return nil
}
