package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Unverified/deghosting/internal/cli"
	"github.com/Unverified/deghosting/internal/deghosting"
	"github.com/Unverified/deghosting/internal/download"
)

func main() {
	os.Exit(run(
		context.Background(),
		os.Args[1:],
		cli.NewInputStream(os.Stdin),
		os.Stdout,
		os.Stderr,
	))
}

func run(
	ctx context.Context,
	args []string,
	stdin cli.InputStream,
	stdout io.Writer,
	stderr io.Writer,
) int {
	command := cli.CLI{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Process: func(ctx context.Context, export io.Reader, out string, ghostURL string, cfg download.Config) error {
			doer := download.NewRetryableHTTPClient(http.DefaultClient)
			op := deghosting.Operation{
				Stdout:     stdout,
				Stderr:     stderr,
				Downloader: download.New(ctx, doer, cfg),
			}
			return op.Process(export, out, ghostURL)
		},
	}

	err := command.Execute(ctx, args)
	return exitCode(err, stderr)
}

func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "deghosting:", err)

	if _, isUsageError := errors.AsType[*cli.UsageError](err); isUsageError {
		return 2
	}
	return 1
}
