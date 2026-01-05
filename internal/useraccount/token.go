package useraccount

import (
	"context"
	"strconv"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	ctx, span := tracer.Start(ctx, "GrantToken",
		trace.WithAttributes(
			attribute.Int("user.id", user.ID),
			attribute.String("user.email", user.Email),
			attribute.String("token.flow", options.flow),
			attribute.Bool("token.impersonation", options.impersonatorID != 0),
		))
	defer span.End()

	span.AddEvent("scopes.fetching")
	// get scopes
	scopes, err := user.QueryGroup().QueryScopeSets().All(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get scopes")
		span.RecordError(err)
		return "", err
	}

	var allScopes []string
	for _, scope := range scopes {
		allScopes = append(allScopes, scope.Scopes...)
	}

	span.SetAttributes(attribute.Int("token.scopes.count", len(allScopes)))

	meta := map[string]string{
		MetaInitiateFromFlow: options.flow,
	}
	if options.impersonatorID != 0 {
		span.AddEvent("event.impersonated.triggered")
		c.eventService.TriggerEvent(ctx, events.Event{
			Type:   events.EventTypeImpersonated,
			UserID: user.ID,
			Payload: map[string]any{
				"impersonator_id": options.impersonatorID,
			},
		})
		meta[MetaImpersonation] = strconv.Itoa(options.impersonatorID)
		span.SetAttributes(attribute.Int("token.impersonator_id", options.impersonatorID))
	} else {
		span.AddEvent("event.login.triggered")
		c.eventService.TriggerEvent(ctx, events.Event{
			Type:   events.EventTypeLogin,
			UserID: user.ID,
			Payload: map[string]any{
				"machine": machine,
			},
		})
		metrics.RecordLogin()
	}

	span.AddEvent("token.create.started")
	token, err := c.auth.Create(ctx, auth.TokenInfo{
		UserID:    user.ID,
		UserEmail: user.Email,
		Machine:   machine,
		Scopes:    allScopes,
		Meta:      meta,
	})
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create token")
		span.RecordError(err)
		return "", err
	}

	span.SetStatus(otelcodes.Ok, "Token granted successfully")
	return token, nil
}

// RevokeToken revokes a token.
func (c *Context) RevokeToken(ctx context.Context, token string) error {
	ctx, span := tracer.Start(ctx, "RevokeToken")
	defer span.End()

	span.AddEvent("token.peek.started")
	tokenInfo, err := c.auth.Peek(ctx, token)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to peek token")
		span.RecordError(err)
		return err
	}

	span.SetAttributes(
		attribute.Int("user.id", tokenInfo.UserID),
		attribute.String("user.email", tokenInfo.UserEmail),
	)

	span.AddEvent("event.logout.triggered")
	c.eventService.TriggerEvent(ctx, events.Event{
		Type:   events.EventTypeLogout,
		UserID: tokenInfo.UserID,
	})

	span.AddEvent("token.delete.started")
	err = c.auth.Delete(ctx, token)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to delete token")
		span.RecordError(err)
		return err
	}

	span.SetStatus(otelcodes.Ok, "Token revoked successfully")
	return nil
}

// RevokeAllTokens revokes all tokens for a user.
func (c *Context) RevokeAllTokens(ctx context.Context, userID int) error {
	ctx, span := tracer.Start(ctx, "RevokeAllTokens",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	span.AddEvent("event.logout_all.triggered")
	c.eventService.TriggerEvent(ctx, events.Event{
		Type:   events.EventTypeLogoutAll,
		UserID: userID,
	})

	span.AddEvent("tokens.delete_by_user.started")
	err := c.auth.DeleteByUser(ctx, userID)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to delete all tokens")
		span.RecordError(err)
		return err
	}

	span.SetStatus(otelcodes.Ok, "All tokens revoked successfully")
	return nil
}
