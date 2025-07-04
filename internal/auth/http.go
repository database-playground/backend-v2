package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/database-playground/backend-v2/graph/defs"
)

// Middleware decodes the Authorization header and packs the user information into context.
//
// It will return 401 if the token is invalid.
func Middleware(storage Storage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			newCtx, err := ExtractToken(r, storage)
			if err != nil {
				gqlgenLikeError(w, err)
				return
			}

			r = r.WithContext(newCtx)
			next.ServeHTTP(w, r)
		})
	}
}

func gqlgenLikeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)

	type StructureError struct {
		Message    string         `json:"message"`
		Path       []string       `json:"path"` // always empty
		Extensions map[string]any `json:"extensions"`
	}

	type Structure struct {
		Errors []StructureError `json:"errors"`
		Data   *struct{}        `json:"data"` // always null
	}

	structure := Structure{
		Errors: []StructureError{
			{
				Message: err.Error(),
				Path:    []string{},
				Extensions: map[string]any{
					"code": defs.CodeUnauthorized,
				},
			},
		},
	}

	encoder.Encode(structure)
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
