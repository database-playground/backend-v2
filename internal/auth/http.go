package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Middleware decodes the Authorization header and packs the user information into context.
//
// It will return 401 if the token is invalid.
func Middleware(storage Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "Middleware",
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
			))
		defer span.End()

		newCtx, err := ExtractToken(c.Request.WithContext(ctx), storage)
		if err != nil {
			var badTokenInfoError BadTokenInfoError
			if errors.As(err, &badTokenInfoError) {
				span.AddEvent("token.revocation")
				// We should revoke the invalid token here.
				if err := storage.Delete(ctx, badTokenInfoError.Token); err != nil {
					span.SetStatus(otelcodes.Error, "Failed to revoke invalid token")
					span.RecordError(err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":  "failed to revoke the token",
						"detail": err.Error(),
					})
					return
				}
			}

			span.AddEvent("auth.failed")
			span.SetStatus(otelcodes.Error, "Authentication failed")
			span.RecordError(err)
			// The standard format for GraphQL errors.
			c.JSON(http.StatusOK, gin.H{
				"errors": []gin.H{
					{
						"message": err.Error(),
						"path":    []string{},
						"extensions": map[string]any{
							"code": defs.CodeUnauthorized,
						},
					},
				},
				"data": nil,
			})
			c.Abort()
			return
		}

		span.AddEvent("auth.succeeded")
		span.SetStatus(otelcodes.Ok, "Authentication successful")
		c.Request = c.Request.WithContext(newCtx)
		c.Next()
	}
}

// ErrBadTokenFormat is returned when the Authorization header is not in the correct Bearer format.
var ErrBadTokenFormat = errors.New("bad token format")

// ExtractToken extracts the token from the Authorization header and returns the user information.
//
// It will return an error if the token is invalid.
// It adds nothing to the context if the token is not present.
func ExtractToken(r *http.Request, storage Storage) (context.Context, error) {
	ctx, span := tracer.Start(r.Context(), "ExtractToken")
	defer span.End()

	authHeaderContent := r.Header.Get("Authorization")
	if authHeaderContent == "" {
		span.SetStatus(otelcodes.Ok, "No authorization header present")
		return ctx, nil
	}

	span.AddEvent("auth.header.found")
	token, ok := strings.CutPrefix(authHeaderContent, "Bearer ")
	if !ok {
		span.SetStatus(otelcodes.Error, "Invalid token format")
		return nil, ErrBadTokenFormat
	}

	span.AddEvent("token.storage.get")
	tokenInfo, err := storage.Get(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			span.AddEvent("token.not_found")
			span.SetStatus(otelcodes.Ok, "Token not found")
			return ctx, nil
		}

		span.SetStatus(otelcodes.Error, "Failed to get token from storage")
		span.RecordError(err)
		return nil, err
	}

	span.AddEvent("token.validation")
	if err := tokenInfo.Validate(); err != nil {
		span.AddEvent("token.validation.failed")
		span.SetStatus(otelcodes.Error, "Token validation failed")
		span.RecordError(err)
		return nil, BadTokenInfoError{
			Token: token,
			Err:   err,
		}
	}

	span.SetAttributes(
		attribute.Int("auth.token.user_id", tokenInfo.UserID),
		attribute.String("auth.token.user_email", tokenInfo.UserEmail),
		attribute.String("auth.token.machine", tokenInfo.Machine),
		attribute.Int("auth.token.scopes.count", len(tokenInfo.Scopes)),
	)
	span.SetStatus(otelcodes.Ok, "Token extracted and validated successfully")
	return WithUser(ctx, tokenInfo), nil
}

// BadTokenInfoError is returned when the token info is invalid.
type BadTokenInfoError struct {
	Token string
	Err   error
}

func (e BadTokenInfoError) Error() string {
	return fmt.Sprintf("bad token info: %v", e.Err)
}

func (e BadTokenInfoError) Unwrap() error {
	return e.Err
}
