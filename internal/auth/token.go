package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateToken generates a random crypto-safe and URL-safe token.
func GenerateToken() (string, error) {
	tokenBytes := make([]byte, 48)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)
	return token, nil
}
