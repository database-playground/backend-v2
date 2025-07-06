package auth

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/internal/testhelper"
)

func TestRedisStorage_Integration(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)

	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	// Test Create and Get
	info := TokenInfo{UserID: 1, UserEmail: "user1@example.com"}
	token, err := storage.Create(ctx, info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if token == "" {
		t.Fatal("Create returned empty token")
	}

	got, err := storage.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.UserID != info.UserID {
		t.Errorf("Get returned wrong user: got %d, want %d", got.UserID, info.UserID)
	}
	if got.UserEmail != info.UserEmail {
		t.Errorf("Get returned wrong user email: got %s, want %s", got.UserEmail, info.UserEmail)
	}

	// Test Delete
	err = storage.Delete(ctx, token)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = storage.Get(ctx, token)
	if err == nil {
		t.Error("Get should fail after Delete, but got no error")
	}

	// Test DeleteByUser
	info2 := TokenInfo{UserID: 2, UserEmail: "user2@example.com"}
	info3 := TokenInfo{UserID: 2, UserEmail: "user2@example.com"}
	token2, err := storage.Create(ctx, info2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token3, err := storage.Create(ctx, info3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = storage.DeleteByUser(ctx, 2)
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}
	_, err = storage.Get(ctx, token2)
	if err == nil {
		t.Error("Get should fail after DeleteByUser for token2, but got no error")
	}
	_, err = storage.Get(ctx, token3)
	if err == nil {
		t.Error("Get should fail after DeleteByUser for token3, but got no error")
	}

	// Test DeleteByUser does not delete other users' tokens
	info4 := TokenInfo{UserID: 3, UserEmail: "user3@example.com"}
	token4, err := storage.Create(ctx, info4)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = storage.DeleteByUser(ctx, 2) // should not affect user3
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}
	_, err = storage.Get(ctx, token4)
	if err != nil {
		t.Errorf("Get failed for user3's token after DeleteByUser for user2: %v", err)
	}
}
