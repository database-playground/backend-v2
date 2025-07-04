package auth

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("no such token")

// Storage is the storage for authentication token.
type Storage interface {
	// Get the token for the given token and (might) extend the expiration time.
	// It returns the token info if the token is valid,
	// otherwise it returns an error.
	//
	// Error is implementation-defined except for ErrNotFound.
	// ErrNotFound is returned when the token is not found.
	Get(ctx context.Context, token string) (TokenInfo, error)

	// Peek the token for the given token. It does not extend the expiration time.
	// It returns the token info if the token is valid,
	// otherwise it returns an error.
	//
	// Error is implementation-defined except for ErrNotFound.
	// ErrNotFound is returned when the token is not found.
	Peek(ctx context.Context, token string) (TokenInfo, error)

	// Create a new token for with the machine name.
	// It returns the refresh token if the creation is successful,
	Create(ctx context.Context, info TokenInfo) (string, error)

	// Delete the specified token.
	Delete(ctx context.Context, token string) error

	// Delete the token for the given user.
	DeleteByUser(ctx context.Context, user string) error
}

// TokenInfo is the information of the token.
type TokenInfo struct {
	Machine string `json:"machine"` // the machine that associated with the token
	User    string `json:"user"`    // the user that associated with the machine

	Scopes []string `json:"scopes"` // the scopes that the user has
}

// DefaultTokenExpire is the default expiration time of the token in seconds.
const DefaultTokenExpire = 8 * 60 * 60 // 8 hr
