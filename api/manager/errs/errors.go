package errs

import (
	"fmt"

	"github.com/pkg/errors"
)

type HTTPStatusError struct {
	StatusCode  int
	Message     string
	OriginalErr error
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("(status %d) %s: %v", e.StatusCode, e.Message, e.OriginalErr)
}

func NewHTTPStatusError(statusCode int, message string, originalErr error) *HTTPStatusError {
	return &HTTPStatusError{
		StatusCode:  statusCode,
		Message:     message,
		OriginalErr: originalErr,
	}
}

func IsHTTPStatusError(err error) (*HTTPStatusError, bool) {
	if err == nil {
		return nil, false
	}
	err = errors.Cause(err)
	httpErr, ok := err.(*HTTPStatusError)
	return httpErr, ok
}
