package sqlrunner

import "errors"

var (
	ErrQueryError    = errors.New("query error")
	ErrSchemaError   = errors.New("schema error")
	ErrBadPayload    = errors.New("payload is invalid")
	ErrInternalError = errors.New("internal error")
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

// ErrorResponse is the error response from the SQL Runner API.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ConvertHttpError converts the error response from
// the SQL Runner API to a more specific error.
func ConvertHttpError(errResp *ErrorResponse) error {
	if errResp == nil {
		return nil
	}

	switch errResp.Code {
	case "QUERY_ERROR":
		return ErrQueryError
	case "SCHEMA_ERROR":
		return ErrSchemaError
	case "BAD_PAYLOAD":
		return ErrBadPayload
	default:
		return ErrInternalError
	}
}
