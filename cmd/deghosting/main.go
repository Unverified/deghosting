package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Unverified/deghosting/internal/cli"
	"github.com/Unverified/deghosting/internal/deghosting"
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
	command := cli.CLI{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,

		Process: deghosting.Operation{
			Stdout: stdout,
			Stderr: stderr,
		}.Process,
	}

	err := command.Execute(args)
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
