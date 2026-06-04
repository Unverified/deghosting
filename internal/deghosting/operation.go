package deghosting

import (
	"fmt"
	"io"
)

// Operation runs the full deghosting workflow and owns user-facing output.
type Operation struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Process converts the Ghost export and reports warnings and a summary.
func (o Operation) Process(export io.Reader, _ string) error {
	stdout := o.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := o.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	result, err := Convert(export)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout,
		"finished, %d posts converted, %d posts skipped, %d warnings\n",
		len(result.Posts),
		len(result.Skipped),
		len(result.Warnings),
	)

	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintln(stderr, "warning:", warning)
	}

	for _, skipped := range result.Skipped {
		_, _ = fmt.Fprintln(stderr, "skipped:", skipped)
	}

	return nil
}
