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

func TestGetUser_EmptyContext(t *testing.T) {
	ctx := context.Background()
	user, ok := auth.GetUser(ctx)
	if ok {
		t.Fatal("expected no user in empty context")
	}
	if user.UserID != 0 {
		t.Fatalf("expected zero user ID, got %d", user.UserID)
	}
}

func TestGetUser_NilContext(t *testing.T) {
	user, ok := auth.GetUser(nil)
	if ok {
		t.Fatal("expected no user in nil context")
	}
	if user.UserID != 0 {
		t.Fatalf("expected zero user ID, got %d", user.UserID)
	}
}

func TestWithUser_CompleteTokenInfo(t *testing.T) {
	ctx := context.Background()
	tokenInfo := auth.TokenInfo{
		UserID:    123,
		UserEmail: "test@example.com",
		Machine:   "test-machine",
		Scopes:    []string{"read", "write"},
		Meta: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	ctx = auth.WithUser(ctx, tokenInfo)
	user, ok := auth.GetUser(ctx)
	if !ok {
		t.Fatal("user not found")
	}

	if user.UserID != tokenInfo.UserID {
		t.Fatalf("expected user ID %d, got %d", tokenInfo.UserID, user.UserID)
	}
	if user.UserEmail != tokenInfo.UserEmail {
		t.Fatalf("expected user email %s, got %s", tokenInfo.UserEmail, user.UserEmail)
	}
	if user.Machine != tokenInfo.Machine {
		t.Fatalf("expected machine %s, got %s", tokenInfo.Machine, user.Machine)
	}
	if len(user.Scopes) != len(tokenInfo.Scopes) {
		t.Fatalf("expected %d scopes, got %d", len(tokenInfo.Scopes), len(user.Scopes))
	}
	if len(user.Meta) != len(tokenInfo.Meta) {
		t.Fatalf("expected %d meta entries, got %d", len(tokenInfo.Meta), len(user.Meta))
	}
}

func TestWithUser_OverwriteExisting(t *testing.T) {
	ctx := context.Background()
	originalTokenInfo := auth.TokenInfo{
		UserID:    1,
		UserEmail: "original@example.com",
		Machine:   "original-machine",
		Scopes:    []string{"original"},
	}

	newTokenInfo := auth.TokenInfo{
		UserID:    2,
		UserEmail: "new@example.com",
		Machine:   "new-machine",
		Scopes:    []string{"new"},
	}

	ctx = auth.WithUser(ctx, originalTokenInfo)
	ctx = auth.WithUser(ctx, newTokenInfo)

	user, ok := auth.GetUser(ctx)
	if !ok {
		t.Fatal("user not found")
	}

	if user.UserID != newTokenInfo.UserID {
		t.Fatalf("expected new user ID %d, got %d", newTokenInfo.UserID, user.UserID)
	}
	if user.UserEmail != newTokenInfo.UserEmail {
		t.Fatalf("expected new user email %s, got %s", newTokenInfo.UserEmail, user.UserEmail)
	}
}

func TestGetUser_ContextWithDifferentType(t *testing.T) {
	ctx := context.Background()
	// Add a different type to the context with the same key
	ctx = context.WithValue(ctx, "user", "not-a-token-info")

	user, ok := auth.GetUser(ctx)
	if ok {
		t.Fatal("expected no user when context has different type")
	}
	if user.UserID != 0 {
		t.Fatalf("expected zero user ID, got %d", user.UserID)
	}
}
