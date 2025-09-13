package authservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestAuthService_RevokeToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful token revocation", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"*"},
		}
		token, err := storage.Create(context.Background(), tokenInfo)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Verify token exists
		_, err = storage.Get(context.Background(), token)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())

		// Verify token was deleted from storage
		_, err = storage.Get(context.Background(), token)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))
	})

	t.Run("successful token revocation without token_type_hint", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"*"},
		}
		token, err := storage.Create(context.Background(), tokenInfo)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request without token_type_hint
		form := url.Values{}
		form.Add("token", token)

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())

		// Verify token was deleted from storage
		_, err = storage.Get(context.Background(), token)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))
	})

	t.Run("revoke non-existent token returns success", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request with non-existent token
		form := url.Values{}
		form.Add("token", "non-existent-token")
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response - should still return 200 OK per RFC 7009
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())
	})

	t.Run("missing token parameter returns error", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request without token parameter
		form := url.Values{}
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "invalid_request", response["error"])
		assert.Equal(t, "Missing required parameter: token", response["error_description"])
	})

	t.Run("empty token parameter returns error", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request with empty token parameter
		form := url.Values{}
		form.Add("token", "")
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "invalid_request", response["error"])
		assert.Equal(t, "Missing required parameter: token", response["error_description"])
	})

	t.Run("unsupported token_type_hint returns error", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request with unsupported token_type_hint
		form := url.Values{}
		form.Add("token", "some-token")
		form.Add("token_type_hint", "refresh_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "unsupported_token_type", response["error"])
		assert.Equal(t, "Only access_token is supported for token_type_hint", response["error_description"])
	})

	t.Run("storage error returns server error", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"*"},
		}
		token, err := storage.Create(context.Background(), tokenInfo)
		require.NoError(t, err)

		// Set storage to return error on delete (not ErrNotFound)
		storageErr := errors.New("database connection failed")
		storage.deleteErr = storageErr

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create revoke request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var response map[string]interface{}
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "server_error", response["error"])
		assert.Equal(t, "Failed to revoke the token. Please try again later.", response["error_description"])
	})

	t.Run("revoke with GET method should not work", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create GET request
		req := httptest.NewRequest("GET", "/auth/v2/revoke?token=some-token&token_type_hint=access_token", nil)
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response - should return 404 since only POST is registered
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("revoke with JSON content type should not work", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Create request with JSON content type
		jsonBody := `{"token": "some-token", "token_type_hint": "access_token"}`
		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response - should return error because token parameter is missing
		// (PostForm doesn't parse JSON)
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "invalid_request", response["error"])
		assert.Equal(t, "Missing required parameter: token", response["error_description"])
	})

	t.Run("revoke multiple tokens sequentially", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create multiple test tokens
		tokens := make([]string, 3)
		for i := 0; i < 3; i++ {
			tokenInfo := auth.TokenInfo{
				UserID:    i + 1,
				UserEmail: "test@example.com",
				Machine:   "test-machine",
				Scopes:    []string{"*"},
			}
			token, err := storage.Create(context.Background(), tokenInfo)
			require.NoError(t, err)
			tokens[i] = token
		}

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Revoke each token
		for _, token := range tokens {
			form := url.Values{}
			form.Add("token", token)
			form.Add("token_type_hint", "access_token")

			req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Verify response
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Empty(t, rr.Body.String())

			// Verify token was deleted
			_, err := storage.Get(context.Background(), token)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, auth.ErrNotFound))
		}
	})

	t.Run("revoke same token twice should succeed both times", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"*"},
		}
		token, err := storage.Create(context.Background(), tokenInfo)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		// First revocation
		req1 := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr1 := httptest.NewRecorder()

		router.ServeHTTP(rr1, req1)

		// Verify first response
		assert.Equal(t, http.StatusOK, rr1.Code)
		assert.Empty(t, rr1.Body.String())

		// Second revocation (token should already be deleted)
		req2 := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr2 := httptest.NewRecorder()

		router.ServeHTTP(rr2, req2)

		// Verify second response - should still succeed per RFC 7009
		assert.Equal(t, http.StatusOK, rr2.Code)
		assert.Empty(t, rr2.Body.String())
	})
}

func TestAuthService_RevokeToken_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("revoke token integration with real storage", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create multiple tokens for different users
		user1TokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "user1@example.com",
			Machine:   "machine-1",
			Scopes:    []string{"*"},
		}
		user1Token, err := storage.Create(context.Background(), user1TokenInfo)
		require.NoError(t, err)

		user2TokenInfo := auth.TokenInfo{
			UserID:    2,
			UserEmail: "user2@example.com",
			Machine:   "machine-2",
			Scopes:    []string{"*"},
		}
		user2Token, err := storage.Create(context.Background(), user2TokenInfo)
		require.NoError(t, err)

		// Verify both tokens exist
		_, err = storage.Get(context.Background(), user1Token)
		require.NoError(t, err)
		_, err = storage.Get(context.Background(), user2Token)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/revoke", authService.RevokeToken)

		// Revoke user1's token
		form := url.Values{}
		form.Add("token", user1Token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify user1's token was deleted but user2's token still exists
		_, err = storage.Get(context.Background(), user1Token)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))

		_, err = storage.Get(context.Background(), user2Token)
		assert.NoError(t, err)
	})
}
