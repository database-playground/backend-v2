package auth

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	// SPEC: 64 bytes, base64 URL-safe, no padding
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if len(token) != 64 {
		t.Fatalf("token length is not 64: %v", token)
	}

	if strings.HasSuffix(token, "=") {
		t.Fatalf("token has padding: %v", token)
	}

	// URL safe characters
	for _, c := range token {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", c) {
			t.Fatalf("token contains invalid character: %v", token)
		}
	}
}

func TestGenerateToken_Fuzz(t *testing.T) {
	s := make(map[string]struct{}, 10000)

	for range 10000 {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		s[token] = struct{}{}
	}

	// make sure all tokens are unique
	if len(s) != 10000 {
		t.Fatalf("some tokens are not unique - %d tokens are generated", len(s))
	}
}
