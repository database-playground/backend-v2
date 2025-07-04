package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		_, err := auth.ExtractToken(&r, nil)
		if !errors.Is(err, auth.ErrNoToken) {
			t.Fatalf("expected no token error, got %v", err)
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
			t.Fatal("handler should not be called")
		})

		middleware := auth.Middleware(storage)
		wrappedHandler := middleware(handler)

		req := &http.Request{
			Header: make(http.Header),
		}
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}

		if !strings.Contains(rr.Body.String(), auth.ErrNoToken.Error()) {
			t.Errorf("expected error message to contain %q, got %q", auth.ErrNoToken.Error(), rr.Body.String())
		}
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

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}

		if !strings.Contains(rr.Body.String(), auth.ErrBadTokenFormat.Error()) {
			t.Errorf("expected error message to contain %q, got %q", auth.ErrBadTokenFormat.Error(), rr.Body.String())
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

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}

		if !strings.Contains(rr.Body.String(), errFailTokenStorage.Error()) {
			t.Errorf("expected error message to contain %q, got %q", errFailTokenStorage.Error(), rr.Body.String())
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
