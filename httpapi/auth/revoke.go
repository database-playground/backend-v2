package authservice

import (
	"errors"
	"net/http"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
)

// RevokeToken implements OAuth 2.0 Token Revocation (RFC 7009)
// POST /api/auth/v2/revoke
func (s *AuthService) RevokeToken(c *gin.Context) {
	// Parse form data
	token := c.PostForm("token")
	tokenTypeHint := c.PostForm("token_type_hint")

	// Validate required parameters
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "Missing required parameter: token",
		})
		return
	}

	// Validate token_type_hint if provided
	if tokenTypeHint != "" && tokenTypeHint != "access_token" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_token_type",
			"error_description": "Only access_token is supported for token_type_hint",
		})
		return
	}

	// Attempt to revoke the token
	err := s.storage.Delete(c.Request.Context(), token)
	if err != nil && !errors.Is(err, auth.ErrNotFound) {
		// Internal server error - failed to revoke token
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": "Failed to revoke the token. Please try again later.",
		})
		return
	}

	// Success - return 200 OK regardless of whether token existed
	// This is per RFC 7009 section 2: "The client must not use the token again after revocation."
	// "The authorization server responds with HTTP status code 200 if the revocation is successful"
	c.Status(http.StatusOK)
}
