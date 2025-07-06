package useraccount

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
)

func (c *Context) GrantToken(ctx context.Context, user *ent.User, machine string, flow string) (string, error) {
	// get scopes
	scopes, err := user.QueryGroup().QueryScopeSet().All(ctx)
	if err != nil {
		return "", err
	}

	var allScopes []string
	for _, scope := range scopes {
		allScopes = append(allScopes, scope.Scopes...)
	}

	token, err := c.auth.Create(ctx, auth.TokenInfo{
		UserID:    user.ID,
		UserEmail: user.Email,
		Machine:   machine,
		Scopes:    allScopes,
		Meta: map[string]string{
			"initiate_from_flow": flow,
		},
	})
	if err != nil {
		return "", err
	}
	return token, nil
}
