package cli

import "errors"

// ErrConversionNotImplemented indicates that command-line parsing and input
// validation succeeded, but the converter itself has not been built yet.
var ErrConversionNotImplemented = errors.New("conversion not implemented")

// UsageError indicates the command line was used incorrectly. The caller maps it
// to exit code 2.
type UsageError struct {
	Message string
	Err     error
}

func (e *UsageError) Error() string {
	if e.Message == "" && e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *UsageError) Unwrap() error {
	return e.Err
}
