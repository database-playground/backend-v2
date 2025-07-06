package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type baseTokenStorage struct{}

// Create implements auth.Storage.
func (r *baseTokenStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	panic("unimplemented")
}

// Delete implements auth.Storage.
func (r *baseTokenStorage) Delete(ctx context.Context, token string) error {
	panic("unimplemented")
}

// DeleteByUser implements auth.Storage.
func (r *baseTokenStorage) DeleteByUser(ctx context.Context, userID int) error {
	panic("unimplemented")
}

// Get implements auth.Storage.
func (r *baseTokenStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	panic("unimplemented")
}

// Peek implements auth.Storage.
func (r *baseTokenStorage) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	panic("unimplemented")
}

var _ auth.Storage = &baseTokenStorage{}

type mockTokenStorage struct {
	baseTokenStorage
	tokenInfo auth.TokenInfo
}

func (m *mockTokenStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	return m.tokenInfo, nil
}

type failTokenStorage struct {
	baseTokenStorage
}

var errFailTokenStorage = errors.New("fail")

func (f *failTokenStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	return auth.TokenInfo{}, errFailTokenStorage
}

type memoryTokenStorage struct {
	storage map[string]auth.TokenInfo
}

func (m *memoryTokenStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	token := uuid.New().String()
	m.storage[token] = info
	return token, nil
}

func (m *memoryTokenStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	info, ok := m.storage[token]
	if !ok {
		return auth.TokenInfo{}, auth.ErrNotFound
	}
	return info, nil
}

func (m *memoryTokenStorage) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	info, ok := m.storage[token]
	if !ok {
		return auth.TokenInfo{}, auth.ErrNotFound
	}
	return info, nil
}

func (m *memoryTokenStorage) Delete(ctx context.Context, token string) error {
	delete(m.storage, token)
	return nil
}

func (m *memoryTokenStorage) DeleteByUser(ctx context.Context, userID int) error {
	for token, info := range m.storage {
		if info.UserID == userID {
			if err := m.Delete(ctx, token); err != nil {
				return fmt.Errorf("delete token: %w", err)
			}
		}
	}
	return nil
}

var _ auth.Storage = &mockTokenStorage{}
var _ auth.Storage = &failTokenStorage{}
var _ auth.Storage = &memoryTokenStorage{}

func TestExtractToken(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		r := http.Request{}
		storage := &mockTokenStorage{}
		ctx, err := auth.ExtractToken(&r, storage)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if ctx != r.Context() {
			t.Fatalf("expected context to be the same, got %v", ctx)
		}
	})

	t.Run("bad token format", func(t *testing.T) {
		r := http.Request{
			Header: http.Header{"Authorization": {"Test 1234"}},
		}
		storage := &mockTokenStorage{}
		_, err := auth.ExtractToken(&r, storage)
		if !errors.Is(err, auth.ErrBadTokenFormat) {
			t.Fatalf("expected bad token error, got %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		r := http.Request{
			Header: http.Header{"Authorization": {"Bearer 1234"}},
		}
		storage := &failTokenStorage{}

		_, err := auth.ExtractToken(&r, storage)
		if !errors.Is(err, errFailTokenStorage) {
			t.Fatalf("expected fail error, got %v", err)
		}
	})

	t.Run("bad token info", func(t *testing.T) {
		storage := &memoryTokenStorage{
			storage: map[string]auth.TokenInfo{},
		}

		tokenInfo := auth.TokenInfo{
			// no user id and email
			Machine: "test",
			Scopes:  []string{"*"},
		}

		token, err := storage.Create(context.Background(), tokenInfo)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		r := http.Request{
			Header: http.Header{"Authorization": {"Bearer " + token}},
		}
		_, err = auth.ExtractToken(&r, storage)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		var badTokenInfoError auth.BadTokenInfoError
		if !errors.As(err, &badTokenInfoError) {
			t.Fatalf("expected bad token info error, got %v", err)
		}
		if badTokenInfoError.Token != token {
			t.Fatalf("expected token %s, got %s", token, badTokenInfoError.Token)
		}
	})

	t.Run("good token", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "user@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		storage := &mockTokenStorage{
			tokenInfo: tokenInfo,
		}

		r := http.Request{
			Header: http.Header{"Authorization": {"Bearer 1234"}},
		}
		ctx, err := auth.ExtractToken(&r, storage)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		user, ok := auth.GetUser(ctx)
		if !ok {
			t.Fatalf("expected user, got none")
		}

		if user.UserID != tokenInfo.UserID {
			t.Fatalf("expected user %d, got %d", tokenInfo.UserID, user.UserID)
		}
		if user.UserEmail != tokenInfo.UserEmail {
			t.Fatalf("expected user email %s, got %s", tokenInfo.UserEmail, user.UserEmail)
		}
	})

	t.Run("good token from cookie", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "user@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		storage := &mockTokenStorage{
			tokenInfo: tokenInfo,
		}

		r := http.Request{
			Header: http.Header{
				"Cookie": []string{fmt.Sprintf("%s=1234", auth.CookieAuthToken)},
			},
		}
		ctx, err := auth.ExtractToken(&r, storage)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		user, ok := auth.GetUser(ctx)
		if !ok {
			t.Fatalf("expected user, got none")
		}

		if user.UserID != tokenInfo.UserID {
			t.Fatalf("expected user %d, got %d", tokenInfo.UserID, user.UserID)
		}
		if user.UserEmail != tokenInfo.UserEmail {
			t.Fatalf("expected user email %s, got %s", tokenInfo.UserEmail, user.UserEmail)
		}
	})
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("no token", func(t *testing.T) {
		storage := &baseTokenStorage{}

		router := gin.New()
		router.Use(auth.Middleware(storage))

		var handlerCalled bool
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true
			_, ok := auth.GetUser(c.Request.Context())
			if ok {
				t.Fatal("expected no user, got one")
			}
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("expected handler to be called")
		}

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	t.Run("bad token format", func(t *testing.T) {
		storage := &baseTokenStorage{}

		router := gin.New()
		router.Use(auth.Middleware(storage))

		router.GET("/test", func(c *gin.Context) {
			t.Error("handler was called when it shouldn't be")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic 1234")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Contains(t, extractGraphqlError(t, rr), auth.ErrBadTokenFormat.Error())
	})

	t.Run("invalid token", func(t *testing.T) {
		storage := &failTokenStorage{}

		router := gin.New()
		router.Use(auth.Middleware(storage))

		router.GET("/test", func(c *gin.Context) {
			t.Error("handler was called when it shouldn't be")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer 1234")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Contains(t, extractGraphqlError(t, rr), errFailTokenStorage.Error())
	})

	t.Run("bad token info", func(t *testing.T) {
		storage := &memoryTokenStorage{
			storage: map[string]auth.TokenInfo{},
		}

		tokenInfo := auth.TokenInfo{
			Machine: "test-machine",
			Scopes:  []string{"*"},
		}

		token, err := storage.Create(context.Background(), tokenInfo)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		router := gin.New()
		router.Use(auth.Middleware(storage))
		router.GET("/test", func(c *gin.Context) {
			t.Error("handler was called when it shouldn't be")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Contains(t, extractGraphqlError(t, rr), "user ID must be positive")

		// check if the token is deleted
		_, err = storage.Get(context.Background(), token)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrNotFound) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test-user@example.com",
			Machine:   "test-machine",
			Scopes:    []string{"*"},
		}
		storage := &mockTokenStorage{
			tokenInfo: tokenInfo,
		}

		router := gin.New()
		router.Use(auth.Middleware(storage))

		var handlerCalled bool
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true

			user, ok := auth.GetUser(c.Request.Context())
			if !ok {
				t.Error("expected user in context, got none")
			}
			if user.UserID != tokenInfo.UserID {
				t.Errorf("expected user %d, got %d", tokenInfo.UserID, user.UserID)
			}
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("expected handler to be called")
		}

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})
}

func extractGraphqlError(t *testing.T, rr *httptest.ResponseRecorder) string {
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected Content-Type %q, got %q", "application/json; charset=utf-8", contentType)
	}

	// Verify error response format
	var response struct {
		Errors []struct {
			Message    string         `json:"message"`
			Path       []string       `json:"path"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
		Data *struct{} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(response.Errors))
	}

	if len(response.Errors[0].Path) != 0 {
		t.Errorf("expected empty path, got %v", response.Errors[0].Path)
	}

	if response.Errors[0].Extensions["code"] != defs.CodeUnauthorized {
		t.Errorf("expected code %q, got %q", defs.CodeUnauthorized, response.Errors[0].Extensions["code"])
	}

	if response.Data != nil {
		t.Error("expected data to be null")
	}

	return response.Errors[0].Message
}

func TestBadTokenInfoError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		originalErr := auth.ErrValidationPositiveUserID
		badTokenErr := auth.BadTokenInfoError{
			Token: "test-token",
			Err:   originalErr,
		}

		expectedMsg := "bad token info: user ID must be positive"
		if badTokenErr.Error() != expectedMsg {
			t.Fatalf("expected error message %q, got %q", expectedMsg, badTokenErr.Error())
		}
	})

	t.Run("error wrapping", func(t *testing.T) {
		originalErr := auth.ErrValidationRequireUserEmail
		badTokenErr := auth.BadTokenInfoError{
			Token: "test-token",
			Err:   originalErr,
		}

		if !errors.Is(badTokenErr, originalErr) {
			t.Fatal("BadTokenInfoError should wrap the original error")
		}
	})

	t.Run("error type assertion", func(t *testing.T) {
		originalErr := auth.ErrValidationRequireMachine
		badTokenErr := auth.BadTokenInfoError{
			Token: "test-token-123",
			Err:   originalErr,
		}

		// Test that we can use errors.As to extract the BadTokenInfoError
		var extractedErr auth.BadTokenInfoError
		if !errors.As(badTokenErr, &extractedErr) {
			t.Fatal("BadTokenInfoError should be extractable with errors.As")
		}

		// Test that the wrapped error is accessible
		if !errors.Is(badTokenErr, originalErr) {
			t.Fatal("BadTokenInfoError should wrap the original error")
		}
	})
}
