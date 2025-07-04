package authutil

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateToken generates a random crypto-safe and URL-safe 64-bytes token.
//
// Example:
//
//	0E3ZZhnnBENG9oz8IeIzbFx0EzyXa_pEK32kjWaZVtliD1SOXsA2gHGeSfwOu_8i
func GenerateToken() (string, error) {
	tokenBytes := make([]byte, 48)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)
	return token, nil
}
