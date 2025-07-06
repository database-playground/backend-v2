package defs

import (
	"fmt"
)

type GqlError struct {
	Message string
	Code    string
}

func (e GqlError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewErrNoSufficientScope(requireScope string) GqlError {
	return GqlError{
		Message: fmt.Sprintf("no sufficient scope: %s", requireScope),
		Code:    CodeForbidden,
	}
}

var ErrNotFound = GqlError{
	Message: "not found",
	Code:    CodeNotFound,
}

var ErrUnauthorized = GqlError{
	Message: "require authentication",
	Code:    CodeUnauthorized,
}

var ErrVerified = GqlError{
	Message: "user already verified",
	Code:    "USER_VERIFIED",
}

const (
	CodeNotFound     = "NOT_FOUND"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
)
