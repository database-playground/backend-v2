package defs

import (
	"fmt"
)

// GqlError is the extension of an error with a code.
type GqlError struct {
	Message string
	Code    string
}

func (e GqlError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewErrNoSufficientScope creates a "no sufficient scope" error.
func NewErrNoSufficientScope(requireScope string) GqlError {
	return GqlError{
		Message: fmt.Sprintf("no sufficient scope: %s", requireScope),
		Code:    CodeUnauthorized,
	}
}

// ErrNotFound is the error for "not found".
var ErrNotFound = GqlError{
	Message: "not found",
	Code:    CodeNotFound,
}

// ErrUnauthorized is the error for "require authentication".
var ErrUnauthorized = GqlError{
	Message: "require authentication",
	Code:    CodeUnauthorized,
}

// ErrVerified is the error for "user already verified".
var ErrVerified = GqlError{
	Message: "user already verified",
	Code:    CodeUserVerified,
}

const (
	// CodeNotFound is the error code for "not found".
	CodeNotFound = "NOT_FOUND"
	// CodeUnauthorized is the error code for "require authentication".
	CodeUnauthorized = "UNAUTHORIZED"
	// CodeUserVerified is the error code for "user already verified".
	CodeUserVerified = "USER_VERIFIED"
)
