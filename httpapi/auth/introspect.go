package authservice

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// IntrospectionResponse represents the OAuth 2.0 token introspection response (RFC 7662)
type IntrospectionResponse struct {
	Active   bool   `json:"active"`
	Username string `json:"username,omitempty"` // user email
	Scope    string `json:"scope,omitempty"`    // space-separated scopes
	Sub      string `json:"sub,omitempty"`      // subject (user ID)
	Exp      int64  `json:"exp,omitempty"`      // expiration time (Unix timestamp)
	Iat      int64  `json:"iat,omitempty"`      // issued at (Unix timestamp)
	Azp      string `json:"azp,omitempty"`      // authorized party (machine name)

	// the acting party to whom authority has been delegated
	Act *IntrospectionAct `json:"act,omitempty"`
}

// IntrospectionAct represents the acting party to whom authority has been delegated
type IntrospectionAct struct {
	Sub string `json:"sub"` // subject (user ID)
}

// IntrospectToken implements OAuth 2.0 Token Introspection (RFC 7662)
// POST /api/auth/v2/introspect
func (s *AuthService) IntrospectToken(c *gin.Context) {
	// Parse form data
	token := c.PostForm("token")
	tokenTypeHint := c.PostForm("token_type_hint")

	ctx, span := tracer.Start(
		c.Request.Context(),
		"IntrospectToken",
		trace.WithAttributes(
			attribute.String("oauth2.token_type_hint", tokenTypeHint),
		),
	)
	defer span.End()

	// Validate required parameters
	if token == "" {
		span.SetStatus(otelcodes.Error, "Missing token parameter")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "Missing required parameter: token",
		})
		return
	}

	// Validate token_type_hint if provided
	if tokenTypeHint != "" && tokenTypeHint != "access_token" {
		span.SetStatus(otelcodes.Error, "Unsupported token type")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_token_type",
			"error_description": "Only access_token is supported for token_type_hint",
		})
		return
	}

	// Try to peek the token (doesn't extend expiration)
	span.AddEvent("oauth2.token.peek")
	tokenInfo, err := s.storage.Peek(ctx, token)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			// Token not found or expired - return inactive token response
			span.SetStatus(otelcodes.Ok, "Token not found or expired")
			c.JSON(http.StatusOK, IntrospectionResponse{
				Active: false,
			})
			return
		}

		// Internal server error
		span.SetStatus(otelcodes.Error, "Storage error")
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": "Failed to introspect the token. Please try again later.",
		})
		return
	}

	span.SetAttributes(
		attribute.Int("user.id", tokenInfo.UserID),
		attribute.StringSlice("oauth2.scopes", tokenInfo.Scopes),
		attribute.String("oauth2.machine", tokenInfo.Machine),
	)

	// Get user information
	span.AddEvent("user.get")
	entUser, err := s.useraccount.GetUser(ctx, tokenInfo.UserID)
	if err != nil {
		if errors.Is(err, useraccount.ErrUserNotFound) {
			// User not found - token is technically invalid
			span.SetStatus(otelcodes.Ok, "User not found, token is invalid")
			c.JSON(http.StatusOK, IntrospectionResponse{
				Active: false,
			})
			return
		}

		// Internal server error
		span.SetStatus(otelcodes.Error, "Failed to get user")
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": err.Error(),
		})
		return
	}

	// Calculate token expiration and issue time
	// Note: This is an approximation since we don't store these explicitly
	// We assume token is valid for DefaultTokenExpire seconds from now
	now := time.Now()
	exp := now.Add(time.Duration(auth.DefaultTokenExpire) * time.Second).Unix()
	iat := now.Unix() // Approximation - we don't have the actual issue time

	// Build successful introspection response
	response := IntrospectionResponse{
		Active:   true,
		Username: entUser.Email,
		Scope:    strings.Join(tokenInfo.Scopes, " "),
		Sub:      strconv.Itoa(tokenInfo.UserID),
		Exp:      exp,
		Iat:      iat,
		Azp:      tokenInfo.Machine,
	}

	if impersonator, ok := tokenInfo.Meta[useraccount.MetaImpersonation]; ok {
		span.SetAttributes(
			attribute.String("oauth2.impersonation.sub", impersonator),
		)
		response.Act = &IntrospectionAct{
			Sub: impersonator,
		}
	}

	span.SetStatus(otelcodes.Ok, "Token introspection completed successfully")
	c.JSON(http.StatusOK, response)
}
