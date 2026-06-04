package cli

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
