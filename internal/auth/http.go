package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Middleware decodes the Authorization header and packs the user information into context.
//
// It will return 401 if the token is invalid.
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
	// ErrBadTokenFormat is returned when the Authorization header is not in the correct Bearer format.
	ErrBadTokenFormat = errors.New("bad token format")
)

// ExtractToken extracts the token from the Authorization header and returns the user information.
//
// It will return an error if the token is invalid.
// It adds nothing to the context if the token is not present.
func ExtractToken(r *http.Request, storage Storage) (context.Context, error) {
	authHeaderContent := r.Header.Get("Authorization")
	if authHeaderContent == "" {
		return r.Context(), nil
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
