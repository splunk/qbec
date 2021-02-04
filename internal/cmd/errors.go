package cmd

import "errors"

// usageError indicates that the user supplied incorrect arguments or flags to the command.
type usageError struct {
	error
}

// NewUsageError returns a usage error
func NewUsageError(msg string) error {
	return &usageError{
		error: errors.New(msg),
	}
}

// IsUsageError returns if the supplied error was caused due to incorrect command usage.
func IsUsageError(err error) bool {
	_, ok := err.(*usageError)
	return ok
}

// runtimeError indicates that there were runtime issues with execution.
type runtimeError struct {
	error
}

// NewRuntimeError returns a runtime error
func NewRuntimeError(err error) error {
	return &runtimeError{
		error: err,
	}
}

// IsRuntimeError returns if the supplied error was a runtime error as opposed to an error arising out of user input.
func IsRuntimeError(err error) bool {
	_, ok := err.(*runtimeError)
	return ok
}

// WrapError passes through usage errors and wraps all other errors with a runtime marker.
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	if IsUsageError(err) {
		return err
	}
	return NewRuntimeError(err)
}
