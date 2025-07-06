package auth_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/internal/auth"
)

func TestUserContext(t *testing.T) {
	ctx := context.Background()
	ctx = auth.WithUser(ctx, auth.TokenInfo{
		UserID:    1,
		UserEmail: "user@example.com",
	})

	user, ok := auth.GetUser(ctx)
	if !ok {
		t.Fatal("user not found")
	}

	if user.UserID != 1 {
		t.Fatalf("user is not correct: %v", user.UserID)
	}

	if user.UserEmail != "user@example.com" {
		t.Fatalf("user email is not correct: %v", user.UserEmail)
	}
}
