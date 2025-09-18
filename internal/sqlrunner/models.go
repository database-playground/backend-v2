package sqlrunner

import (
	"fmt"
)

// QueryRequest is the request to the SQL Runner API.
type QueryRequest struct {
	Schema string `json:"schema"`
	Query  string `json:"query"`
}

// QueryResponse is the response from the SQL Runner API.
type QueryResponse struct {
	*SuccessResponse // only success = true
	*ErrorResponse   // only success = false

	Success bool `json:"success"`
}

// SuccessResponse is the response from the SQL Runner API.
type SuccessResponse struct {
	Data DataResponse `json:"data"`
}

// DataResponse is the data response from the SQL Runner API.
type DataResponse struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

const (
	ErrorCodeQueryError    = "QUERY_ERROR"
	ErrorCodeSchemaError   = "SCHEMA_ERROR"
	ErrorCodeBadPayload    = "BAD_PAYLOAD"
	ErrorCodeInternalError = "INTERNAL_ERROR"
)

// ErrorResponse is the error response from the SQL Runner API.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
