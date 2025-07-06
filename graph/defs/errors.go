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

var ErrNotFound = GqlError{
	Message: "not found",
	Code:    CodeNotFound,
}
var ErrUnauthorized = GqlError{
	Message: "require authentication",
	Code:    CodeUnauthorized,
}
var ErrNoSufficientScope = GqlError{
	Message: "no sufficient scope",
	Code:    CodeForbidden,
}

const CodeNotFound = "NOT_FOUND"
const CodeUnauthorized = "UNAUTHORIZED"
const CodeForbidden = "FORBIDDEN"
