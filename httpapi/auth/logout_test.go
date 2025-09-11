package authservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// mockAuthStorage implements auth.Storage for testing
type mockAuthStorage struct {
	tokens    map[string]auth.TokenInfo
	deleteErr error
}

func newMockAuthStorage() *mockAuthStorage {
	return &mockAuthStorage{
		tokens: make(map[string]auth.TokenInfo),
	}
}

func (m *mockAuthStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	token := "test-token-" + info.UserEmail + "-" + info.Machine
	m.tokens[token] = info
	return token, nil
}

func (m *mockAuthStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	info, exists := m.tokens[token]
	if !exists {
		return auth.TokenInfo{}, auth.ErrNotFound
	}
	return info, nil
}

func (m *mockAuthStorage) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	return m.Get(ctx, token)
}

func (m *mockAuthStorage) Delete(ctx context.Context, token string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, exists := m.tokens[token]; !exists {
		return auth.ErrNotFound
	}
	delete(m.tokens, token)
	return nil
}

func (m *mockAuthStorage) DeleteByUser(ctx context.Context, userID int) error {
	for token, info := range m.tokens {
		if info.UserID == userID {
			delete(m.tokens, token)
		}
	}
	return nil
}

func setupTestAuthService(t *testing.T) (*AuthService, *mockAuthStorage) {
	entClient := testhelper.NewEntSqliteClient(t)
	storage := newMockAuthStorage()
	cfg := config.Config{}

	authService := NewAuthService(entClient, storage, cfg)
	return authService, storage
}

func TestAuthService_Logout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful logout with valid token", func(t *testing.T) {
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

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Create request with cookie
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: token,
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)

		// Verify token was deleted from storage
		_, err = storage.Get(context.Background(), token)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))

		// Verify cookie was cleared
		cookies := rr.Result().Cookies()
		var authCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == auth.CookieAuthToken {
				authCookie = cookie
				break
			}
		}
		require.NotNil(t, authCookie)
		assert.Equal(t, "", authCookie.Value)
		assert.Equal(t, -1, authCookie.MaxAge)
		assert.Equal(t, "/", authCookie.Path)
		assert.True(t, authCookie.Secure)
		assert.True(t, authCookie.HttpOnly)
	})

	t.Run("logout without token should do nothing", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Create request without cookie
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)
	})

	t.Run("logout with invalid token should just revoke", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Create request with invalid token
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: "invalid-token",
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)

		// Verify token was deleted from response
		cookies := rr.Result().Cookies()
		var authCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == auth.CookieAuthToken {
				authCookie = cookie
				break
			}
		}
		require.NotNil(t, authCookie)
		assert.Equal(t, "", authCookie.Value)
		assert.Equal(t, -1, authCookie.MaxAge)
		assert.Equal(t, "/", authCookie.Path)
		assert.True(t, authCookie.Secure)
		assert.True(t, authCookie.HttpOnly)
	})

	t.Run("logout with storage error returns internal server error", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Set storage to return error on delete
		storageErr := errors.New("storage connection failed")
		storage.deleteErr = storageErr

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
		router.POST("/auth/logout", authService.Logout)

		// Create request with cookie
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: token,
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		// Verify error message
		var response map[string]interface{}
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "Failed to revoke the token. Please try again later.", response["error"])
		assert.Equal(t, storageErr.Error(), response["detail"])
	})

	t.Run("logout with malformed cookie should do nothing", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Create request with malformed cookie
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.Header.Set("Cookie", "malformed-cookie")
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)
	})

	t.Run("logout sets SameSite cookie attribute", func(t *testing.T) {
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
		router.POST("/auth/logout", authService.Logout)

		// Create request with cookie
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: token,
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)

		// Verify SameSite attribute is set
		cookies := rr.Result().Cookies()
		var authCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == auth.CookieAuthToken {
				authCookie = cookie
				break
			}
		}
		require.NotNil(t, authCookie)
		assert.Equal(t, http.SameSiteStrictMode, authCookie.SameSite)
	})

	t.Run("logout with empty token value should just revoke", func(t *testing.T) {
		authService, _ := setupTestAuthService(t)

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Create request with empty token
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: "",
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)
	})

	t.Run("logout with multiple cookies handles correctly", func(t *testing.T) {
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
		router.POST("/auth/logout", authService.Logout)

		// Create request with multiple cookies
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  "other-cookie",
			Value: "other-value",
		})
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: token,
		})
		rr := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)

		// Verify token was deleted from storage
		_, err = storage.Get(context.Background(), token)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))

		// Verify only auth cookie was cleared
		cookies := rr.Result().Cookies()
		var authCookie *http.Cookie
		var otherCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == auth.CookieAuthToken {
				authCookie = cookie
			} else if cookie.Name == "other-cookie" {
				otherCookie = cookie
			}
		}
		require.NotNil(t, authCookie)
		assert.Equal(t, "", authCookie.Value)
		assert.Nil(t, otherCookie) // Other cookie should not be affected
	})
}

func TestAuthService_Logout_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("logout flow with real storage", func(t *testing.T) {
		authService, storage := setupTestAuthService(t)

		// Create multiple tokens for the same user
		tokenInfo1 := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "machine-1",
			Scopes:    []string{"*"},
		}
		token1, err := storage.Create(context.Background(), tokenInfo1)
		require.NoError(t, err)

		tokenInfo2 := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "machine-2",
			Scopes:    []string{"*"},
		}
		token2, err := storage.Create(context.Background(), tokenInfo2)
		require.NoError(t, err)

		// Verify both tokens exist
		_, err = storage.Get(context.Background(), token1)
		require.NoError(t, err)
		_, err = storage.Get(context.Background(), token2)
		require.NoError(t, err)

		// Setup router
		router := gin.New()
		router.POST("/auth/logout", authService.Logout)

		// Logout with token1
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieAuthToken,
			Value: token1,
		})
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusResetContent, rr.Code)

		// Verify token1 was deleted but token2 still exists
		_, err = storage.Get(context.Background(), token1)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrNotFound))

		_, err = storage.Get(context.Background(), token2)
		assert.NoError(t, err)
	})
}
