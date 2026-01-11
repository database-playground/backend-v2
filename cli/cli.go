// Package cli provides the CLI service for the backend.
package cli

import (
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/internal/submission"
)

// Context is the context for the CLI.
type Context struct {
	entClient         *ent.Client
	submissionService *submission.SubmissionService
}

// NewContext creates a new Context with all services.
func NewContext(entClient *ent.Client, eventService *events.EventService, sqlrunner *sqlrunner.SqlRunner) *Context {
	return &Context{
		entClient:         entClient,
		submissionService: submission.NewSubmissionService(entClient, eventService, sqlrunner),
	}
}
