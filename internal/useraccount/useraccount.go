// Package useraccount manages the user account and its lifecycle.
package useraccount

import (
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
)

type Context struct {
	entClient    *ent.Client
	auth         auth.Storage
	eventService *events.EventService
}

func NewContext(entClient *ent.Client, auth auth.Storage, eventService *events.EventService) *Context {
	return &Context{
		entClient:    entClient,
		auth:         auth,
		eventService: eventService,
	}
}
