package defs

import "errors"

var ErrUnauthorized = errors.New("require authentication")   // -> 401
var ErrNoSufficientScope = errors.New("no sufficient scope") // -> 403

const CodeUnauthorized = "UNAUTHORIZED"
const CodeForbidden = "FORBIDDEN"
