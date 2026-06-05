// Package cli implements deghosting's command-line interface: argument parsing,
// input/output resolution, and the scaffold execution flow.
package cli

import (
	"context"
	"errors"
	"io"
	"runtime"

	"github.com/Unverified/deghosting/internal/download"
	"github.com/alecthomas/kong"
)

// ProcessFunc runs the process after the CLI has parsed arguments and
// resolved the export input stream.
type ProcessFunc func(ctx context.Context, export io.Reader, out string, ghostURL string, cfg download.Config) error

// CLI is the wired command-line application.
type CLI struct {
	Process ProcessFunc
	Stdin   InputStream
	Stdout  io.Writer
	Stderr  io.Writer
}

// Execute parses args and runs the wired process. It never calls os.Exit; the
// caller maps the returned error to an exit code.
func (c CLI) Execute(ctx context.Context, args []string) error {
	if c.Process == nil {
		return errors.New("process function is nil")
	}
	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	var options Options
	parser, err := kong.New(&options,
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

	if err = ValidateOutputDir(options.Out, options.Force); err != nil {
		return err
	}

	input, err := ResolveInput(options.Input, options.ExportJSON, c.Stdin)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	concurrency := options.AssetConcurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}

	cfg := download.Config{
		Concurrency: concurrency,
	}

	return c.Process(ctx, input, options.Out, options.GhostURL, cfg)
}
