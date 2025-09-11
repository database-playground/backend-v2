package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/gin-gonic/gin"
)

const CookieAuthToken = "__Host-Auth-Token"

// Middleware decodes the Authorization header and packs the user information into context.
//
// It will return 401 if the token is invalid.
func Middleware(storage Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		newCtx, err := ExtractToken(c.Request, storage)
		if err != nil {
			var badTokenInfoError BadTokenInfoError
			if errors.As(err, &badTokenInfoError) {
				// We should revoke the invalid token here.
				if err := storage.Delete(c.Request.Context(), badTokenInfoError.Token); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":  "failed to revoke the token",
						"detail": err.Error(),
					})
					return
				}
			}

			// The standard format for GraphQL errors.
			c.JSON(http.StatusOK, gin.H{
				"errors": []gin.H{
					{
						"message": err.Error(),
						"path":    []string{},
						"extensions": map[string]any{
							"code": defs.CodeUnauthorized,
						},
					},
				},
				"data": nil,
			})
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(newCtx)
		c.Next()
	}
}

var (
	// ErrNoTokenFound is returned when no token is found.
	ErrNoTokenFound = errors.New("no token found")

	// ErrBadTokenFormat is returned when the Authorization header is not in the correct Bearer format.
	ErrBadTokenFormat = errors.New("bad token format")
)

// ExtractToken extracts the token from the Authorization header and returns the user information.
//
// It will return an error if the token is invalid.
// It adds nothing to the context if the token is not present.
func ExtractToken(r *http.Request, storage Storage) (context.Context, error) {
	type TokenSource func(r *http.Request) (string, error)

	tokenSources := []TokenSource{
		// Header: Authorization: Bearer <token>
		func(r *http.Request) (string, error) {
			authHeaderContent := r.Header.Get("Authorization")
			if authHeaderContent == "" {
				return "", ErrNoTokenFound
			}

			token, ok := strings.CutPrefix(authHeaderContent, "Bearer ")
			if !ok {
				return "", ErrBadTokenFormat
			}

			return token, nil
		},

		// Cookies: __Host-Auth-Token=<token>
		func(r *http.Request) (string, error) {
			cookie, err := r.Cookie(CookieAuthToken)
			if err != nil {
				if errors.Is(err, http.ErrNoCookie) {
					return "", ErrNoTokenFound
				}

				return "", err
			}

			return cookie.Value, nil
		},
	}

	for _, tokenSource := range tokenSources {
		token, err := tokenSource(r)
		if errors.Is(err, ErrNoTokenFound) {
			continue // try next token source
		}
		if err != nil {
			return nil, err
		}

		tokenInfo, err := storage.Get(r.Context(), token)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return r.Context(), nil
			}

			return nil, err
		}

		if err := tokenInfo.Validate(); err != nil {
			return nil, BadTokenInfoError{
				Token: token,
				Err:   err,
			}
		}

		return WithUser(r.Context(), tokenInfo), nil
	}

	return r.Context(), nil
}

// BadTokenInfoError is returned when the token info is invalid.
type BadTokenInfoError struct {
	Token string
	Err   error
}

func (e BadTokenInfoError) Error() string {
	return fmt.Sprintf("bad token info: %v", e.Err)
}

func (e BadTokenInfoError) Unwrap() error {
	return e.Err
}
