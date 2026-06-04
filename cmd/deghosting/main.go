package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Unverified/deghosting/internal/cli"
)

func main() {
	os.Exit(run(
		os.Args[1:],
		cli.NewInputStream(os.Stdin),
		os.Stdout,
		os.Stderr,
	))
}

func run(
	args []string,
	stdin cli.InputStream,
	stdout io.Writer,
	stderr io.Writer,
) int {
	err := cli.Execute(args, stdin, stdout, stderr)
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
