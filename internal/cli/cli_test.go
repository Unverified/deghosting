package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Unverified/deghosting/internal/cli"
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

// panicReader fails the test if anything tries to read from it.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("input was read before output validation")
}

func TestExecuteValidatesOutputBeforeReadingInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.md"))

	// No --input and a non-terminal stdin would resolve input to "-", so a
	// failing output dir must be rejected before the stdin reader is touched.
	err := cli.Execute(
		[]string{"-o", dir},
		cli.InputStream{Reader: panicReader{}},
		io.Discard,
		io.Discard,
	)
	require.Error(t, err)
}

func TestExecuteMapsParseErrorsToUsageError(t *testing.T) {
	t.Parallel()

	err := cli.Execute(
		[]string{"--definitely-not-a-flag"},
		cli.InputStream{Reader: strings.NewReader(""), IsTerminal: true},
		io.Discard,
		io.Discard,
	)

	var usageErr *cli.UsageError
	require.ErrorAs(t, err, &usageErr)
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	writeFileContent(t, path, "x")
}

func writeFileContent(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
