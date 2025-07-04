package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/database-playground/backend-v2/internal/testhelper"
)

func TestRedisStorage_CreateAndGet(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

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
}

func TestRedisStorage_Get_InvalidToken(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	_, err := storage.Get(ctx, "nonexistent-token")
	if err == nil {
		t.Error("Get should fail for nonexistent token, but got no error")
	}
}

func TestRedisStorage_Delete(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info := TokenInfo{Machine: "machine2", User: "user2"}
	token, err := storage.Create(ctx, info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = storage.Delete(ctx, token)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = storage.Get(ctx, token)
	if err == nil {
		t.Error("Get should fail after Delete, but got no error")
	}
}

func TestRedisStorage_DeleteByUser(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info1 := TokenInfo{Machine: "machine3", User: "user3"}
	info2 := TokenInfo{Machine: "machine4", User: "user3"}
	info3 := TokenInfo{Machine: "machine5", User: "user4"}
	token1, err := storage.Create(ctx, info1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token2, err := storage.Create(ctx, info2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	token3, err := storage.Create(ctx, info3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = storage.DeleteByUser(ctx, "user3")
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}

	_, err = storage.Get(ctx, token1)
	if err == nil {
		t.Error("Get should fail after DeleteByUser for token1, but got no error")
	}
	_, err = storage.Get(ctx, token2)
	if err == nil {
		t.Error("Get should fail after DeleteByUser for token2, but got no error")
	}
	// token3 should still exist
	_, err = storage.Get(ctx, token3)
	if err != nil {
		t.Errorf("Get failed for user4's token after DeleteByUser for user3: %v", err)
	}
}

func TestRedisStorage_DeleteByUser_Cursor(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	const user = "bulkuser"
	const otherUser = "otheruser"
	const tokenCount = 1200 // Large enough to require multiple SCAN iterations

	tokens := make([]string, 0, tokenCount)
	for i := 0; i < tokenCount; i++ {
		info := TokenInfo{Machine: fmt.Sprintf("machine-bulk-%d", i), User: user}
		token, err := storage.Create(ctx, info)
		if err != nil {
			t.Fatalf("Create failed at %d: %v", i, err)
		}
		tokens = append(tokens, token)
	}

	// Create a few tokens for another user, which should not be deleted
	otherTokens := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		info := TokenInfo{Machine: fmt.Sprintf("machine-other-%d", i), User: otherUser}
		token, err := storage.Create(ctx, info)
		if err != nil {
			t.Fatalf("Create failed for other user at %d: %v", i, err)
		}
		otherTokens = append(otherTokens, token)
	}

	err := storage.DeleteByUser(ctx, user)
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}

	// All tokens for 'user' should be deleted
	for i, token := range tokens {
		_, err := storage.Get(ctx, token)
		if err == nil {
			t.Errorf("Get should fail after DeleteByUser for token %d, but got no error", i)
		}
	}

	// Tokens for 'otherUser' should still exist
	for i, token := range otherTokens {
		_, err := storage.Get(ctx, token)
		if err != nil {
			t.Errorf("Get failed for otherUser's token %d after DeleteByUser for bulkuser: %v", i, err)
		}
	}
}
