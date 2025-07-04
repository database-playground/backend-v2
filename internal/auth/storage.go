package auth

import "context"

// Storage is the storage for authentication token.
type Storage interface {
	// Get the token for the given token.
	// It returns the machine name if the refresh token is valid,
	// otherwise it returns an error.
	Get(ctx context.Context, token string) (TokenInfo, error)

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
}
