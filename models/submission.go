package models

import "github.com/database-playground/backend-v2/internal/sqlrunner"

// SubmissionResult is the result of a submission.
type SubmissionResult struct {
	Version  string                  `json:"version"` // version 1 is the only supported version
	Response sqlrunner.QueryResponse `json:"response"`
}
