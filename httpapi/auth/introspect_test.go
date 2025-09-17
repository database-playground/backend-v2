package authservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// mockAuthStorageForIntrospect is a specific mock for introspect tests
type mockAuthStorageForIntrospect struct {
	*mockAuthStorage
	peekErr error
}

func newMockAuthStorageForIntrospect() *mockAuthStorageForIntrospect {
	return &mockAuthStorageForIntrospect{
		mockAuthStorage: newMockAuthStorage(),
	}
}

func (m *mockAuthStorageForIntrospect) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	if m.peekErr != nil {
		return auth.TokenInfo{}, m.peekErr
	}
	return m.mockAuthStorage.Peek(ctx, token)
}

func setupTestAuthServiceWithDatabase(t *testing.T) (*AuthService, *mockAuthStorageForIntrospect, *ent.Client) {
	entClient := testhelper.NewEntSqliteClient(t)

	// Setup database with required groups and scope sets
	_, err := setup.Setup(context.Background(), entClient)
	require.NoError(t, err)

	storage := newMockAuthStorageForIntrospect()
	cfg := config.Config{}
	eventService := events.NewEventService(entClient)
	useraccount := useraccount.NewContext(entClient, storage, eventService)

	authService := NewAuthService(entClient, storage, cfg, useraccount)
	return authService, storage, entClient
}

func TestAuthService_IntrospectToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful token introspection with valid token", func(t *testing.T) {
		authService, storage, entClient := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a group for the user
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ("unverified")).Only(ctx)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("Test User").
			SetEmail("test@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    user.ID,
			UserEmail: "test@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"read", "write"},
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Active)
		assert.Equal(t, "test@example.com", response.Username)
		assert.Equal(t, "read write", response.Scope)
		assert.Equal(t, strconv.Itoa(user.ID), response.Sub)
		assert.Equal(t, "test-machine", response.Azp)
		assert.Greater(t, response.Exp, int64(0))
		assert.Greater(t, response.Iat, int64(0))
	})

	t.Run("successful token introspection with admin scope", func(t *testing.T) {
		authService, storage, entClient := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a group for the user
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ("unverified")).Only(ctx)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("Admin User").
			SetEmail("admin@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create a test token with admin scope
		tokenInfo := auth.TokenInfo{
			UserID:    user.ID,
			UserEmail: "admin@example.com",
			Machine:   "admin-machine",
			Scopes:    []string{"*"}, // Admin scope
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Active)
		assert.Equal(t, "admin@example.com", response.Username)
		assert.Equal(t, "*", response.Scope) // Admin has all permissions
		assert.Equal(t, strconv.Itoa(user.ID), response.Sub)
		assert.Equal(t, "admin-machine", response.Azp)
	})

	t.Run("successful token introspection without token_type_hint", func(t *testing.T) {
		authService, storage, entClient := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a group for the user
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ("unverified")).Only(ctx)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("Test User").
			SetEmail("test2@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create a test token
		tokenInfo := auth.TokenInfo{
			UserID:    user.ID,
			UserEmail: "test2@example.com",
			Machine:   "test-machine-2",
			Scopes:    []string{"read"},
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request without token_type_hint
		form := url.Values{}
		form.Add("token", token)

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Active)
		assert.Equal(t, "test2@example.com", response.Username)
		assert.Equal(t, "read", response.Scope)
	})

	t.Run("inactive token response for non-existent token", func(t *testing.T) {
		authService, _, _ := setupTestAuthServiceWithDatabase(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request with non-existent token
		form := url.Values{}
		form.Add("token", "non-existent-token")
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.False(t, response.Active)
		assert.Empty(t, response.Username)
		assert.Empty(t, response.Scope)
		assert.Empty(t, response.Sub)
		assert.Empty(t, response.Azp)
		assert.Equal(t, int64(0), response.Exp)
		assert.Equal(t, int64(0), response.Iat)
	})

	t.Run("inactive token response when user does not exist", func(t *testing.T) {
		authService, storage, _ := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a test token for a non-existent user
		tokenInfo := auth.TokenInfo{
			UserID:    999999, // Non-existent user ID
			UserEmail: "nonexistent@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"read"},
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response - should return inactive when user doesn't exist
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.False(t, response.Active)
	})

	t.Run("missing token parameter returns error", func(t *testing.T) {
		authService, _, _ := setupTestAuthServiceWithDatabase(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request without token parameter
		form := url.Values{}
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
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
		authService, _, _ := setupTestAuthServiceWithDatabase(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request with empty token parameter
		form := url.Values{}
		form.Add("token", "")
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
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
		authService, _, _ := setupTestAuthServiceWithDatabase(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request with unsupported token_type_hint
		form := url.Values{}
		form.Add("token", "test-token")
		form.Add("token_type_hint", "refresh_token") // Unsupported type

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
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
		authService, storage, _ := setupTestAuthServiceWithDatabase(t)

		// Set up storage to return an error
		storage.peekErr = errors.New("database connection failed")

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", "test-token")
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "server_error", response["error"])
		assert.Equal(t, "Failed to introspect the token. Please try again later.", response["error_description"])
	})

	t.Run("successful token introspection with impersonation (act field)", func(t *testing.T) {
		authService, storage, entClient := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a group for the user
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ("unverified")).Only(ctx)
		require.NoError(t, err)

		// Create a test user (the impersonated user)
		user, err := entClient.User.Create().
			SetName("Impersonated User").
			SetEmail("impersonated@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create an impersonator user
		impersonator, err := entClient.User.Create().
			SetName("Admin User").
			SetEmail("admin@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create a test token with impersonation metadata
		tokenInfo := auth.TokenInfo{
			UserID:    user.ID,
			UserEmail: "impersonated@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"read", "write"},
			Meta: map[string]string{
				"impersonation": strconv.Itoa(impersonator.ID),
			},
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		// Verify basic token information
		assert.True(t, response.Active)
		assert.Equal(t, "impersonated@example.com", response.Username)
		assert.Equal(t, "read write", response.Scope)
		assert.Equal(t, strconv.Itoa(user.ID), response.Sub)
		assert.Equal(t, "test-machine", response.Azp)
		assert.Greater(t, response.Exp, int64(0))
		assert.Greater(t, response.Iat, int64(0))

		// Verify the act field is populated with impersonator information
		require.NotNil(t, response.Act, "Act field should be populated when impersonation is present")
		assert.Equal(t, strconv.Itoa(impersonator.ID), response.Act.Sub, "Act.Sub should contain the impersonator's user ID")
	})

	t.Run("successful token introspection without impersonation (no act field)", func(t *testing.T) {
		authService, storage, entClient := setupTestAuthServiceWithDatabase(t)
		ctx := context.Background()

		// Create a group for the user
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ("unverified")).Only(ctx)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("Regular User").
			SetEmail("regular@example.com").
			SetGroup(unverifiedGroup).
			Save(ctx)
		require.NoError(t, err)

		// Create a test token without impersonation metadata
		tokenInfo := auth.TokenInfo{
			UserID:    user.ID,
			UserEmail: "regular@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"read", "write"},
			Meta:      map[string]string{}, // No impersonation metadata
		}
		token, err := storage.Create(ctx, tokenInfo)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Setup router
		router := gin.New()
		router.POST("/auth/v2/introspect", authService.IntrospectToken)

		// Create introspect request
		form := url.Values{}
		form.Add("token", token)
		form.Add("token_type_hint", "access_token")

		req := httptest.NewRequest("POST", "/auth/v2/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)

		var response IntrospectionResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		// Verify basic token information
		assert.True(t, response.Active)
		assert.Equal(t, "regular@example.com", response.Username)
		assert.Equal(t, "read write", response.Scope)
		assert.Equal(t, strconv.Itoa(user.ID), response.Sub)
		assert.Equal(t, "test-machine", response.Azp)

		// Verify the act field is NOT populated when there's no impersonation
		assert.Nil(t, response.Act, "Act field should be nil when no impersonation is present")
	})
}
