// Package useraccount manages the user account and its lifecycle.
package useraccount

import (
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
)

type Context struct {
	entClient *ent.Client
	auth      auth.Storage
}

func NewContext(entClient *ent.Client, auth auth.Storage) *Context {
	return &Context{
		entClient: entClient,
		auth:      auth,
	}
}
