// Package useraccount manages the user account and its lifecycle.
package useraccount

import (
	"github.com/database-playground/backend-v2/ent"
)

type Context struct {
	entClient *ent.Client
}

func NewContext(entClient *ent.Client) *Context {
	return &Context{
		entClient: entClient,
	}
}
