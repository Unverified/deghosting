// Package cli implements deghosting's command-line interface: argument parsing,
// input/output resolution, and the scaffold execution flow.
package cli

import (
	"io"

	"github.com/alecthomas/kong"
)

// Execute parses args and runs the scaffold flow. It never calls os.Exit; the
// caller maps the returned error to an exit code.
func Execute(args []string, stdin InputStream, stdout, stderr io.Writer) error {
	var cli CLI
	parser, err := kong.New(&cli,
		kong.Name("deghosting"),
		kong.Description("Convert a Ghost export into a Markdown content tree."),
		kong.Writers(stdout, stderr),
	)
	if err != nil {
		return err
	}

	if _, err = parser.Parse(args); err != nil {
		return &UsageError{Message: err.Error(), Err: err}
	}

	// Validate the output directory before opening input, so we fail fast instead
	// of blocking on stdin when the command is already doomed.
	if err = ValidateOutputDir(cli.Out, cli.Force); err != nil {
		return err
	}

	input, err := ResolveInput(cli.Input, cli.ExportJSON, stdin)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	return ErrConversionNotImplemented
}
