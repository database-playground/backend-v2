package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Middleware decodes the share session cookie and packs the session into context
func Middleware(storage Storage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			newCtx, err := ExtractToken(r, storage)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			r = r.WithContext(newCtx)
			next.ServeHTTP(w, r)
		})
	}
}

var (
	ErrNoToken        = errors.New("no token")
	ErrBadTokenFormat = errors.New("bad token format")
)

func ExtractToken(r *http.Request, storage Storage) (context.Context, error) {
	authHeaderContent := r.Header.Get("Authorization")
	if authHeaderContent == "" {
		return nil, ErrNoToken
	}

	token, ok := strings.CutPrefix(authHeaderContent, "Bearer ")
	if !ok {
		return nil, ErrBadTokenFormat
	}

	tokenInfo, err := storage.Get(r.Context(), token)
	if err != nil {
		return nil, err
	}

	return WithUser(r.Context(), tokenInfo), nil
}
