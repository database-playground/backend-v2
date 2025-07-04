package auth_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/internal/auth"
)

func TestUserContext(t *testing.T) {
	ctx := context.Background()
	ctx = auth.WithUser(ctx, auth.TokenInfo{
		Machine: "machine",
		User:    "user",
	})

	user, ok := auth.GetUser(ctx)
	if !ok {
		t.Fatal("user not found")
	}

	if user.Machine != "machine" {
		t.Fatalf("machine is not correct: %v", user.Machine)
	}

	if user.User != "user" {
		t.Fatalf("user is not correct: %v", user.User)
	}
}
