package auth

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/internal/testhelper"
)

func TestRedisStorage_CreateAndGet(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info := TokenInfo{
		UserID:    1,
		UserEmail: "user1@example.com",
		Scopes:    []string{"*"},
		Meta: map[string]string{
			"key": "value",
		},
	}
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
	if !slices.Equal(got.Scopes, info.Scopes) {
		t.Errorf("Get returned wrong scopes: got %v, want %v", got.Scopes, info.Scopes)
	}
	if !maps.Equal(got.Meta, info.Meta) {
		t.Errorf("Get returned wrong meta: got %v, want %v", got.Meta, info.Meta)
	}
}

func TestRedisStorage_CreateAndGet_Expire(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient, WithTokenExpire(1*time.Second))
	ctx := context.Background()

	info := TokenInfo{UserID: 1, UserEmail: "user1@example.com"}

	token, err := storage.Create(ctx, info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// wait 2 seconds to make sure the token is expired
	time.Sleep(2 * time.Second)

	// token should be expired
	_, err = storage.Get(ctx, token)
	if err == nil {
		t.Error("Get should fail after 1 second, but got no error")
	}
}

func TestRedisStorage_Get_InvalidToken(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	_, err := storage.Get(ctx, "nonexistent-token")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get should fail for nonexistent token, but got %v", err)
	}
}

func TestRedisStorage_Delete(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info := TokenInfo{UserID: 2, UserEmail: "user2@example.com"}
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

func TestRedisStorage_Delete_InvalidToken(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	err := storage.Delete(ctx, "nonexistent-token")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete should return ErrNotFound for nonexistent token, but got %v", err)
	}
}

func TestRedisStorage_DeleteByUser(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info1 := TokenInfo{UserID: 3, UserEmail: "user3@example.com"}
	info2 := TokenInfo{UserID: 3, UserEmail: "user3@example.com"}
	info3 := TokenInfo{UserID: 4, UserEmail: "user4@example.com"}
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

	err = storage.DeleteByUser(ctx, 3)
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

	const tokenCount = 1200 // Large enough to require multiple SCAN iterations

	tokens := make([]string, 0, tokenCount)
	for i := 0; i < tokenCount; i++ {
		info := TokenInfo{UserID: 1, UserEmail: "user-1@example.com"}
		token, err := storage.Create(ctx, info)
		if err != nil {
			t.Fatalf("Create failed at %d: %v", i, err)
		}
		tokens = append(tokens, token)
	}

	// Create a few tokens for another user, which should not be deleted
	otherTokens := make([]string, 0, 3)
	for i := range 3 {
		info := TokenInfo{UserID: i + 2, UserEmail: fmt.Sprintf("other-user-%d@example.com", i)}
		token, err := storage.Create(ctx, info)
		if err != nil {
			t.Fatalf("Create failed for other user at %d: %v", i, err)
		}
		otherTokens = append(otherTokens, token)
	}

	err := storage.DeleteByUser(ctx, 1)
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

func TestRedisStorage_Peek(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)
	storage := NewRedisStorage(redisClient)
	ctx := context.Background()

	info := TokenInfo{UserID: 1, UserEmail: "user1@example.com"}
	token, err := storage.Create(ctx, info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get current TTL
	ttl, err := storage.GetCurrentTTL(ctx, token)
	if err != nil {
		t.Fatalf("GetExpiration failed: %v", err)
	}

	// Peek should not affect expiration
	got, err := storage.Peek(ctx, token)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if got.UserID != info.UserID {
		t.Errorf("Peek returned wrong user: got %d, want %d", got.UserID, info.UserID)
	}

	time.Sleep(2 * time.Second) // make sure afterPeekExpire < (ttl + latency)

	afterPeekExpire, err := storage.GetCurrentTTL(ctx, token)
	if err != nil {
		t.Fatalf("GetCurrentTTL after peek failed: %v", err)
	}
	if afterPeekExpire >= ttl {
		t.Errorf("GetCurrentTTL after peek should be less than to ttl, but got %d", afterPeekExpire)
	}

	// Regular Get should update expiration
	_, err = storage.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	afterGetExpire, err := storage.GetCurrentTTL(ctx, token)
	if err != nil {
		t.Fatalf("GetCurrentTTL after get failed: %v", err)
	}
	if afterGetExpire <= afterPeekExpire {
		t.Errorf("GetCurrentTTL after get should be greater than afterPeekExpire, but got %d", afterGetExpire)
	}
	if afterGetExpire > ttl {
		t.Errorf("GetCurrentTTL after get should be less than or equal to ttl, but got %d", afterGetExpire)
	}
}

func TestTestRedisStorage_GetCurrentTTL(t *testing.T) {
	container := testhelper.NewRedisContainer(t)
	redisClient := testhelper.NewRedisClient(t, container)

	const testTTLSec = 10
	storage := NewRedisStorage(redisClient, WithTokenExpire(time.Duration(testTTLSec)*time.Second))
	ctx := context.Background()

	info := TokenInfo{UserID: 1, UserEmail: "user1@example.com"}
	token, err := storage.Create(ctx, info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ttl, err := storage.GetCurrentTTL(ctx, token)
	if err != nil {
		t.Fatalf("GetCurrentTTL failed: %v", err)
	}

	if ttl < testTTLSec {
		t.Errorf("Ttl should be less than %d, but got %d", testTTLSec, ttl)
	}
}
