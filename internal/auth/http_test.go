package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/internal/auth"
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
func (r *baseTokenStorage) DeleteByUser(ctx context.Context, user string) error {
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

var _ auth.Storage = &mockTokenStorage{}
var _ auth.Storage = &failTokenStorage{}

func TestExtractToken(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		r := http.Request{}
		ctx, err := auth.ExtractToken(&r, nil)
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
		_, err := auth.ExtractToken(&r, nil)
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

	t.Run("good token", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			Machine: "machine",
			User:    "user",
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

		if user.Machine != tokenInfo.Machine {
			t.Fatalf("expected machine %s, got %s", tokenInfo.Machine, user.Machine)
		}

		if user.User != tokenInfo.User {
			t.Fatalf("expected user %s, got %s", tokenInfo.User, user.User)
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		storage := &baseTokenStorage{}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := auth.GetUser(r.Context())
			if ok {
				t.Fatal("expected no user, got one")
			}
		})

		middleware := auth.Middleware(storage)
		wrappedHandler := middleware(handler)

		req := &http.Request{
			Header: make(http.Header),
		}
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)
	})

	t.Run("bad token format", func(t *testing.T) {
		storage := &baseTokenStorage{}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})

		middleware := auth.Middleware(storage)
		wrappedHandler := middleware(handler)

		req := &http.Request{
			Header: http.Header{
				"Authorization": []string{"Basic 1234"},
			},
		}
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		// Check content type
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
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

		if response.Errors[0].Message != auth.ErrBadTokenFormat.Error() {
			t.Errorf("expected error message %q, got %q", auth.ErrBadTokenFormat.Error(), response.Errors[0].Message)
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
	})

	t.Run("invalid token", func(t *testing.T) {
		storage := &failTokenStorage{}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})

		middleware := auth.Middleware(storage)
		wrappedHandler := middleware(handler)

		req := &http.Request{
			Header: http.Header{
				"Authorization": []string{"Bearer 1234"},
			},
		}
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		// Check content type
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
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

		if response.Errors[0].Message != errFailTokenStorage.Error() {
			t.Errorf("expected error message %q, got %q", errFailTokenStorage.Error(), response.Errors[0].Message)
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
	})

	t.Run("valid token", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			Machine: "test-machine",
			User:    "test-user",
		}
		storage := &mockTokenStorage{
			tokenInfo: tokenInfo,
		}

		var handlerCalled bool
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true

			// Verify user info in context
			user, ok := auth.GetUser(r.Context())
			if !ok {
				t.Error("expected user in context, got none")
			}
			if user.Machine != tokenInfo.Machine {
				t.Errorf("expected machine %q, got %q", tokenInfo.Machine, user.Machine)
			}
			if user.User != tokenInfo.User {
				t.Errorf("expected user %q, got %q", tokenInfo.User, user.User)
			}
		})

		middleware := auth.Middleware(storage)
		wrappedHandler := middleware(handler)

		req := &http.Request{
			Header: http.Header{
				"Authorization": []string{"Bearer valid-token"},
			},
		}
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("expected handler to be called")
		}

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})
}
