package errors

import "errors"

var (
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrParseFailed   = errors.New("parse failed")
	ErrLoadFailed    = errors.New("load failed")
	ErrDiffFailed    = errors.New("diff failed")
)
