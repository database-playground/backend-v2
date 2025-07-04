package gauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/database-playground/backend-v2/internal/testhelper"
)

func TestTestRedisStateStorage_GetCurrentTTL(t *testing.T) {
	ctx := context.Background()
	redisContainer := testhelper.NewRedisContainer(t)
	redis := testhelper.NewRedisClient(t, redisContainer)

	storage := &TestRedisStateStorage{
		RedisStateStorage: NewRedisTokenStorage(redis),
	}

	t.Run("returns TTL for valid token", func(t *testing.T) {
		// Create a new token
		token, err := storage.New(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Get TTL
		ttl, err := storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)

		// TTL should be close to stateTokenExpire (10 minutes)
		assert.InDelta(t, int64(stateTokenExpire.Seconds()), ttl, 5, "TTL should be close to 10 minutes")
	})

	t.Run("returns -2 for non-existent token", func(t *testing.T) {
		ttl, err := storage.GetCurrentTTL(ctx, "non-existent-token")
		require.NoError(t, err)
		assert.Equal(t, int64(-2), ttl, "TTL should be -2 for non-existent key")
	})

	t.Run("returns -1 for token with no expiry", func(t *testing.T) {
		token := "test-token-no-expiry"
		err := redis.Do(ctx, redis.B().Set().Key(stateTokenPrefix+token).Value(token).Build()).Error()
		require.NoError(t, err)

		ttl, err := storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, int64(-1), ttl, "TTL should be -1 for key with no expiry")

		// Cleanup
		_ = redis.Do(ctx, redis.B().Del().Key(stateTokenPrefix+token).Build()).Error()
	})
}

func TestTestRedisStateStorage_Integration(t *testing.T) {
	ctx := context.Background()
	redisContainer := testhelper.NewRedisContainer(t)
	redis := testhelper.NewRedisClient(t, redisContainer)

	storage := &TestRedisStateStorage{
		RedisStateStorage: NewRedisTokenStorage(redis),
	}

	t.Run("full lifecycle test", func(t *testing.T) {
		// Create new token
		token, err := storage.New(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Check TTL exists
		ttl, err := storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.True(t, ttl > 0, "TTL should be positive")

		// Use token
		err = storage.Use(ctx, token)
		require.NoError(t, err)

		// Verify token is deleted
		ttl, err = storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, int64(-2), ttl, "TTL should be -2 after token is used")
	})

	t.Run("concurrent token creation and usage", func(t *testing.T) {
		// Create multiple tokens
		tokens := make([]string, 3)
		for i := range tokens {
			token, err := storage.New(ctx)
			require.NoError(t, err)
			tokens[i] = token
		}

		// Verify all tokens exist with TTL
		for _, token := range tokens {
			ttl, err := storage.GetCurrentTTL(ctx, token)
			require.NoError(t, err)
			assert.True(t, ttl > 0, "TTL should be positive")
		}

		// Use all tokens
		for _, token := range tokens {
			err := storage.Use(ctx, token)
			require.NoError(t, err)
		}

		// Verify all tokens are deleted
		for _, token := range tokens {
			ttl, err := storage.GetCurrentTTL(ctx, token)
			require.NoError(t, err)
			assert.Equal(t, int64(-2), ttl, "TTL should be -2 after token is used")
		}
	})

	t.Run("token expiration", func(t *testing.T) {
		// This test requires a mock Redis client to properly test expiration
		// without waiting for actual time to pass. For now, we'll just verify
		// that the token is created with the correct expiration time.
		token, err := storage.New(ctx)
		require.NoError(t, err)

		ttl, err := storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.InDelta(t, int64(stateTokenExpire.Seconds()), ttl, 5, "TTL should be close to 10 minutes")
	})
}

func TestRedisStateStorage_New(t *testing.T) {
	ctx := context.Background()
	redisContainer := testhelper.NewRedisContainer(t)
	redis := testhelper.NewRedisClient(t, redisContainer)

	storage := NewRedisTokenStorage(redis)

	t.Run("creates new token with TTL", func(t *testing.T) {
		// Create token
		token, err := storage.New(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Verify token exists in Redis
		result := redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		require.NoError(t, result.Error())

		// Verify token value matches
		val, err := result.ToString()
		require.NoError(t, err)
		assert.Equal(t, token, val)

		// Verify TTL is set
		ttlCmd := redis.Do(ctx, redis.B().Ttl().Key(stateTokenPrefix+token).Build())
		require.NoError(t, ttlCmd.Error())
		ttl, err := ttlCmd.AsInt64()
		require.NoError(t, err)
		assert.InDelta(t, int64(stateTokenExpire.Seconds()), ttl, 5)
	})

	t.Run("creates unique tokens", func(t *testing.T) {
		// Create multiple tokens
		token1, err := storage.New(ctx)
		require.NoError(t, err)
		token2, err := storage.New(ctx)
		require.NoError(t, err)

		assert.NotEqual(t, token1, token2)
	})
}

func TestRedisStateStorage_Use(t *testing.T) {
	ctx := context.Background()
	redisContainer := testhelper.NewRedisContainer(t)
	redis := testhelper.NewRedisClient(t, redisContainer)

	storage := NewRedisTokenStorage(redis)

	t.Run("successfully uses and deletes token", func(t *testing.T) {
		// Create and store token
		token, err := storage.New(ctx)
		require.NoError(t, err)

		// Verify token exists
		result := redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		require.NoError(t, result.Error())

		// Use token
		err = storage.Use(ctx, token)
		require.NoError(t, err)

		// Verify token is deleted
		result = redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		assert.Error(t, result.Error())
	})

	t.Run("returns ErrBadState for non-existent token", func(t *testing.T) {
		err := storage.Use(ctx, "non-existent-token")
		assert.ErrorIs(t, err, ErrBadState)
	})

	t.Run("token can only be used once", func(t *testing.T) {
		// Create token
		token, err := storage.New(ctx)
		require.NoError(t, err)

		// First use should succeed
		err = storage.Use(ctx, token)
		require.NoError(t, err)

		// Second use should fail
		err = storage.Use(ctx, token)
		assert.ErrorIs(t, err, ErrBadState)
	})
}
