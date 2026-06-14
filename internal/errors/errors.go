package errors

import (
	"errors"
	"fmt"
)

// Error is a custom error type that supports context and wrapping.
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// WithCause wraps an existing error with additional context.
func (e *Error) WithCause(cause error) *Error {
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Cause:   cause,
	}
}

// WithMessage returns a new error with an updated message.
func (e *Error) WithMessage(msg string) *Error {
	return &Error{
		Code:    e.Code,
		Message: msg,
		Cause:   e.Cause,
	}
}

// Sentinel errors for backward compatibility.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrParseFailed   = errors.New("parse failed")
	ErrLoadFailed    = errors.New("load failed")
	ErrDiffFailed    = errors.New("diff failed")
)

// Structured errors with codes for new code paths.
var (
	ErrCodeNotFound    = &Error{Code: "NOT_FOUND", Message: "resource not found"}
	ErrCodeParseFailed = &Error{Code: "PARSE_FAILED", Message: "parse failed"}
	ErrCodeLoadFailed  = &Error{Code: "LOAD_FAILED", Message: "load failed"}
	ErrCodeDiffFailed  = &Error{Code: "DIFF_FAILED", Message: "diff failed"}
)
