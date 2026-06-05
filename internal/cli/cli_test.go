package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Unverified/deghosting/internal/cli"
	"github.com/Unverified/deghosting/internal/download"
	"github.com/stretchr/testify/require"
)

func TestResolveInputSuccess(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "ghost.json")
	writeFileContent(t, file, "file-data")

	tests := []struct {
		name        string
		flagInput   string
		positionals []string
		stdin       cli.InputStream
		want        string
	}{
		{
			name:      "input flag only",
			flagInput: file,
			want:      "file-data",
		},
		{
			name:        "positional only",
			positionals: []string{file},
			want:        "file-data",
		},
		{
			name: "no input with piped stdin",
			stdin: cli.InputStream{
				Reader: strings.NewReader("stdin-data"),
			},
			want: "stdin-data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rc, err := cli.ResolveInput(tc.flagInput, tc.positionals, tc.stdin)
			require.NoError(t, err)
			defer func() { _ = rc.Close() }()

			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

func TestResolveInputUsageErrors(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "ghost.json")
	writeFileContent(t, file, "file-data")

	tests := []struct {
		name        string
		flagInput   string
		positionals []string
		stdin       cli.InputStream
	}{
		{
			name:        "flag and positional conflict",
			flagInput:   file,
			positionals: []string{file},
		},
		{
			name:        "too many positionals",
			positionals: []string{file, file},
		},
		{
			name:  "no input with terminal stdin",
			stdin: cli.InputStream{Reader: strings.NewReader(""), IsTerminal: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rc, err := cli.ResolveInput(tc.flagInput, tc.positionals, tc.stdin)

			var usageErr *cli.UsageError
			require.ErrorAs(t, err, &usageErr)
			require.Nil(t, rc)
		})
	}
}

func TestValidateOutputDir(t *testing.T) {
	t.Parallel()

	t.Run("missing dir is ok", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "does-not-exist")
		require.NoError(t, cli.ValidateOutputDir(path, false))
	})

	t.Run("empty dir is ok", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, cli.ValidateOutputDir(t.TempDir(), false))
	})

	t.Run("non-empty dir fails without force", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "post.md"))
		require.Error(t, cli.ValidateOutputDir(dir, false))
	})

	t.Run("non-empty dir ok with force", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "post.md"))
		require.NoError(t, cli.ValidateOutputDir(dir, true))
	})

	t.Run("regular file path fails", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "content")
		writeFile(t, path)
		require.Error(t, cli.ValidateOutputDir(path, false))
	})
}

func TestExecutePassesResolvedInputToProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "ghost.json")
	writeFileContent(t, file, "file-data")

	var stdout bytes.Buffer

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader("stdin-data"), IsTerminal: true},
		Stdout: &stdout,
		Stderr: io.Discard,
		Process: func(_ context.Context, export io.Reader, out string, _ string, _ download.Config) error {
			require.Equal(t, dir, out)
			got, err := io.ReadAll(export)
			require.NoError(t, err)
			require.Equal(t, "file-data", string(got))
			return nil
		},
	}

	err := command.Execute(t.Context(), []string{"--input", file, "--out", dir, "--force"})
	require.NoError(t, err)
	require.Empty(t, stdout.String())
}

func TestExecutePassesGhostURLToProcess(t *testing.T) {
	t.Parallel()

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader("data"), IsTerminal: false},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(_ context.Context, _ io.Reader, _ string, ghostURL string, _ download.Config) error {
			require.Equal(t, "https://blog.example.com", ghostURL)
			return nil
		},
	}

	err := command.Execute(t.Context(), []string{"--ghost-url", "https://blog.example.com"})
	require.NoError(t, err)
}

func TestExecuteAssetFlagDefaults(t *testing.T) {
	t.Parallel()

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader("data"), IsTerminal: false},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(_ context.Context, _ io.Reader, _ string, _ string, cfg download.Config) error {
			require.Equal(t, runtime.NumCPU(), cfg.Concurrency)
			return nil
		},
	}

	err := command.Execute(t.Context(), nil)
	require.NoError(t, err)
}

func TestExecuteAssetFlagOverrides(t *testing.T) {
	t.Parallel()

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader("data"), IsTerminal: false},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(_ context.Context, _ io.Reader, _ string, _ string, cfg download.Config) error {
			require.Equal(t, 4, cfg.Concurrency)
			return nil
		},
	}

	err := command.Execute(t.Context(), []string{"--asset-concurrency", "4"})
	require.NoError(t, err)
}

func TestExecutePropagatesProcessError(t *testing.T) {
	t.Parallel()

	want := errors.New("process failed")
	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader("stdin-data"), IsTerminal: false},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(context.Context, io.Reader, string, string, download.Config) error {
			return want
		},
	}

	err := command.Execute(t.Context(), nil)

	require.ErrorIs(t, err, want)
}

func TestExecuteMapsParseErrorsToUsageError(t *testing.T) {
	t.Parallel()

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: strings.NewReader(""), IsTerminal: true},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(context.Context, io.Reader, string, string, download.Config) error {
			t.Fatal("process should not be called for parse errors")
			return nil
		},
	}
	err := command.Execute(t.Context(), []string{"--definitely-not-a-flag"})

	var usageErr *cli.UsageError
	require.ErrorAs(t, err, &usageErr)
}

// panicReader fails the test if anything tries to read from it.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("input was read before output validation")
}

func TestExecuteValidatesOutputBeforeProcessing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.md"))

	command := cli.CLI{
		Stdin:  cli.InputStream{Reader: panicReader{}, IsTerminal: false},
		Stdout: io.Discard,
		Stderr: io.Discard,
		Process: func(context.Context, io.Reader, string, string, download.Config) error {
			t.Fatal("process should not be called when output validation fails")
			return nil
		},
	}

	err := command.Execute(t.Context(), []string{"--out", dir})

	require.Error(t, err)
}

func TestExecuteRequiresProcessFunction(t *testing.T) {
	t.Parallel()

	err := cli.CLI{
		Stdin: cli.InputStream{Reader: strings.NewReader("stdin-data"), IsTerminal: false},
	}.Execute(t.Context(), nil)

	require.ErrorContains(t, err, "process function is nil")
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	writeFileContent(t, path, "x")
}

func writeFileContent(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
