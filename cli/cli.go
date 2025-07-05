// Package cli provides the CLI service for the backend.
package cli

import (
	"github.com/database-playground/backend-v2/ent"
)

// Context is the context for the CLI.
type Context struct {
	entClient *ent.Client
}

// NewContext creates a new Context.
func NewContext(entClient *ent.Client) *Context {
	return &Context{
		entClient: entClient,
	}
}
