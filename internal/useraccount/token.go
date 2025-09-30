package useraccount

import (
	"context"
	"strconv"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
)

const (
	MetaInitiateFromFlow = "initiate_from_flow"
	MetaImpersonation    = "impersonation"
)

type grantTokenOptions struct {
	flow           string
	impersonatorID int
}

type GrantTokenOption func(*grantTokenOptions)

func WithFlow(flow string) GrantTokenOption {
	return func(o *grantTokenOptions) {
		o.flow = flow
	}
}

func WithImpersonation(impersonatorID int) GrantTokenOption {
	return func(o *grantTokenOptions) {
		o.impersonatorID = impersonatorID
	}
}

// GrantToken creates a new token for the user.
func (c *Context) GrantToken(ctx context.Context, user *ent.User, machine string, opts ...GrantTokenOption) (string, error) {
	options := &grantTokenOptions{
		flow:           "undefined",
		impersonatorID: 0,
	}
	for _, opt := range opts {
		opt(options)
	}

	// get scopes
	scopes, err := user.QueryGroup().QueryScopeSets().All(ctx)
	if err != nil {
		return "", err
	}

	var allScopes []string
	for _, scope := range scopes {
		allScopes = append(allScopes, scope.Scopes...)
	}

	meta := map[string]string{
		MetaInitiateFromFlow: options.flow,
	}
	if options.impersonatorID != 0 {
		c.eventService.TriggerEvent(ctx, events.Event{
			Type:   events.EventTypeImpersonated,
			UserID: user.ID,
			Payload: map[string]any{
				"impersonator_id": options.impersonatorID,
			},
		})
		meta[MetaImpersonation] = strconv.Itoa(options.impersonatorID)
	} else {
		c.eventService.TriggerEvent(ctx, events.Event{
			Type:   events.EventTypeLogin,
			UserID: user.ID,
			Payload: map[string]any{
				"machine": machine,
			},
		})
	}

	token, err := c.auth.Create(ctx, auth.TokenInfo{
		UserID:    user.ID,
		UserEmail: user.Email,
		Machine:   machine,
		Scopes:    allScopes,
		Meta:      meta,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// RevokeToken revokes a token.
func (c *Context) RevokeToken(ctx context.Context, token string) error {
	tokenInfo, err := c.auth.Peek(ctx, token)
	if err != nil {
		return err
	}

	c.eventService.TriggerEvent(ctx, events.Event{
		Type:   events.EventTypeLogout,
		UserID: tokenInfo.UserID,
	})

	return c.auth.Delete(ctx, token)
}

// RevokeAllTokens revokes all tokens for a user.
func (c *Context) RevokeAllTokens(ctx context.Context, userID int) error {
	c.eventService.TriggerEvent(ctx, events.Event{
		Type:   events.EventTypeLogoutAll,
		UserID: userID,
	})

	return c.auth.DeleteByUser(ctx, userID)
}
