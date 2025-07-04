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
	info := TokenInfo{Machine: "machine1", User: "user1"}
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
	if got != info {
		t.Errorf("Get returned wrong info: got %+v, want %+v", got, info)
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
	info2 := TokenInfo{Machine: "machine2", User: "user2"}
	info3 := TokenInfo{Machine: "machine3", User: "user2"}
	token2, err := storage.Create(ctx, info2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token3, err := storage.Create(ctx, info3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = storage.DeleteByUser(ctx, "user2")
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
	info4 := TokenInfo{Machine: "machine4", User: "user3"}
	token4, err := storage.Create(ctx, info4)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = storage.DeleteByUser(ctx, "user2") // should not affect user3
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}
	_, err = storage.Get(ctx, token4)
	if err != nil {
		t.Errorf("Get failed for user3's token after DeleteByUser for user2: %v", err)
	}
}
