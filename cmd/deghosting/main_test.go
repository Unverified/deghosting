package main

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/Unverified/deghosting/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", err: nil, want: 0},
		{name: "usage error", err: &cli.UsageError{Message: "bad usage"}, want: 2},
		{name: "wrapped usage error", err: fmt.Errorf("context: %w", &cli.UsageError{Message: "bad usage"}), want: 2},
		{name: "runtime error", err: errors.New("boom"), want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, exitCode(tc.err, io.Discard))
		})
	}
}
