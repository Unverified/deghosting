package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// CLI describes the deghosting command line.
type CLI struct {
	Input string `short:"i" name:"input" help:"Ghost export JSON file, or '-' for stdin."`
	Out   string `short:"o" name:"out" default:"content" help:"Output content directory."`
	Force bool   `short:"f" name:"force" help:"Allow writing into a non-empty output directory."`

	ExportJSON []string `arg:"" optional:"" name:"export.json" help:"Ghost export JSON file."`
}

// InputStream is the command's stdin plus the terminal state needed to decide
// whether missing file arguments should fall back to piped input.
type InputStream struct {
	Reader     io.Reader
	IsTerminal bool
}

// NewInputStream describes stdin for production callers backed by an *os.File.
func NewInputStream(stdin *os.File) InputStream {
	info, err := stdin.Stat()
	isTerminal := err == nil && info.Mode()&os.ModeCharDevice != 0

	return InputStream{
		Reader:     stdin,
		IsTerminal: isTerminal,
	}
}

// ResolveInput selects the input source from the --input flag and any positional
// argument, opening the named file or returning stdin for "-". An explicit "-"
// is honored unconditionally. Conflicting or excess inputs, and a missing input
// with no piped stdin, are UsageErrors. The caller must Close the result.
func ResolveInput(flagInput string, positionalInputs []string, stdin InputStream) (io.ReadCloser, error) {
	path, err := resolveInputPath(flagInput, positionalInputs, stdin.IsTerminal)
	if err != nil {
		return nil, err
	}

	if path == "-" {
		if stdin.Reader == nil {
			return nil, errors.New("stdin reader is nil")
		}
		return io.NopCloser(stdin.Reader), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}

	return f, nil
}

// resolveInputPath determines the input path ("-" for stdin) from the --input
// flag and positional arguments, without touching the filesystem.
func resolveInputPath(flagInput string, positionalInputs []string, stdinIsTerminal bool) (string, error) {
	if len(positionalInputs) > 1 {
		return "", &UsageError{Message: "at most one positional input may be given"}
	}

	var positional string
	if len(positionalInputs) == 1 {
		positional = positionalInputs[0]
	}

	switch {
	case flagInput != "" && positional != "":
		return "", &UsageError{Message: "specify input with --input or a positional argument, not both"}
	case flagInput != "":
		return flagInput, nil
	case positional != "":
		return positional, nil
	case !stdinIsTerminal:
		return "-", nil
	default:
		return "", &UsageError{Message: "no input provided: pass --input, a file argument, or pipe data on stdin"}
	}
}

// ValidateOutputDir checks that path is usable as the output directory. A missing
// path or an empty directory is fine; a non-empty directory requires force; a
// regular file at path is always an error. Nothing is created or deleted.
func ValidateOutputDir(path string, force bool) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat output directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("output path %q is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read output directory: %w", err)
	}

	if len(entries) > 0 && !force {
		return fmt.Errorf("output directory %q is not empty (use --force to write into it)", path)
	}

	return nil
}
