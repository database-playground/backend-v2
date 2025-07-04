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

var ErrUnauthorized = GqlError{
	Message: "require authentication",
	Code:    CodeUnauthorized,
}
var ErrNoSufficientScope = GqlError{
	Message: "no sufficient scope",
	Code:    CodeForbidden,
}

const CodeUnauthorized = "UNAUTHORIZED"
const CodeForbidden = "FORBIDDEN"
