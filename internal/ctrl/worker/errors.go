package worker

import (
	"fmt"
)

type ProcessError struct {
	message   string
	retryable bool
}

func (re *ProcessError) Error() string {
	return re.message
}

func (re *ProcessError) Retryable() bool {
	return re.retryable
}

func (re *ProcessError) Is(err error) bool {
	re2, ok := err.(*ProcessError)
	if !ok {
		return false
	}
	return re.retryable == re2.retryable
}

var RetryableError = &ProcessError{retryable: true}
var UnretryableError = &ProcessError{retryable: false}

func NewProcessError(retryable bool, format string, a ...any) *ProcessError {
	return &ProcessError{
		message:   fmt.Sprintf(format, a...),
		retryable: retryable,
	}
}
