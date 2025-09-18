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

// ErrForbidden is the error for "no access to this resource".
var ErrForbidden = GqlError{
	Message: "no access to this resource",
	Code:    CodeForbidden,
}

// ErrVerified is the error for "user already verified".
var ErrVerified = GqlError{
	Message: "user already verified",
	Code:    CodeUserVerified,
}

// ErrNotImplemented is the error for "not implemented".
var ErrNotImplemented = GqlError{
	Message: "not implemented",
	Code:    CodeNotImplemented,
}

var ErrDisallowUpdateGroup = GqlError{
	Message: "update group of yourself is not allowed",
	Code:    CodeForbidden,
}

var ErrInvalidFilter = GqlError{
	Message: "invalid filter",
	Code:    CodeInvalidInput,
}

const (
	// CodeNotFound is the error code for "not found".
	CodeNotFound = "NOT_FOUND"
	// CodeUnauthorized is the error code for "require authentication".
	CodeUnauthorized = "UNAUTHORIZED"
	// CodeUserVerified is the error code for "user already verified".
	CodeUserVerified = "USER_VERIFIED"
	// CodeNotImplemented is the error code for "not implemented".
	CodeNotImplemented = "NOT_IMPLEMENTED"
	// CodeForbidden is the error code for "forbidden".
	CodeForbidden = "FORBIDDEN"
	// CodeInvalidInput is the error code for "invalid input".
	CodeInvalidInput = "INVALID_INPUT"
)
