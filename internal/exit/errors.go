package exit

import "fmt"

// CodedError carries a wrapper exit Code alongside an error, so the CLI top level
// can translate it into the process exit status.
type CodedError struct {
	Code Code
	Err  error
}

func (e *CodedError) Error() string {
	if e.Err == nil {
		return e.Code.String()
	}
	return e.Err.Error()
}

func (e *CodedError) Unwrap() error { return e.Err }

// Errorf builds a CodedError with a formatted message.
func Errorf(code Code, format string, args ...any) *CodedError {
	return &CodedError{Code: code, Err: fmt.Errorf(format, args...)}
}

// Wrap attaches a code to an existing error (nil-safe: returns nil for nil err).
func Wrap(code Code, err error) *CodedError {
	if err == nil {
		return nil
	}
	return &CodedError{Code: code, Err: err}
}
