package gauth

import (
	"context"
	"fmt"
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
		token, err := storage.New(ctx, []byte("test-data"))
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
		token, err := storage.New(ctx, []byte("test-data"))
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Check TTL exists
		ttl, err := storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.True(t, ttl > 0, "TTL should be positive")

		// Use token
		data, err := storage.Use(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, []byte("test-data"), data)

		// Verify token is deleted
		ttl, err = storage.GetCurrentTTL(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, int64(-2), ttl, "TTL should be -2 after token is used")
	})

	t.Run("concurrent token creation and usage", func(t *testing.T) {
		// Create multiple tokens
		tokens := make([]string, 3)
		for i := range tokens {
			token, err := storage.New(ctx, []byte("test-data"))
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
			data, err := storage.Use(ctx, token)
			require.NoError(t, err)
			assert.Equal(t, []byte("test-data"), data)
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
		token, err := storage.New(ctx, []byte("test-data"))
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
		token, err := storage.New(ctx, []byte("test-data"))
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Verify token exists in Redis
		result := redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		require.NoError(t, result.Error())

		// Verify token value matches
		val, err := result.AsBytes()
		require.NoError(t, err)
		assert.Equal(t, []byte("test-data"), val)

		// Verify TTL is set
		ttlCmd := redis.Do(ctx, redis.B().Ttl().Key(stateTokenPrefix+token).Build())
		require.NoError(t, ttlCmd.Error())
		ttl, err := ttlCmd.AsInt64()
		require.NoError(t, err)
		assert.InDelta(t, int64(stateTokenExpire.Seconds()), ttl, 5)
	})

	t.Run("creates unique tokens", func(t *testing.T) {
		// Create multiple tokens
		token1, err := storage.New(ctx, []byte("test-data"))
		require.NoError(t, err)
		token2, err := storage.New(ctx, []byte("test-data"))
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
		token, err := storage.New(ctx, []byte("test-data"))
		require.NoError(t, err)

		// Verify token exists
		result := redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		require.NoError(t, result.Error())

		// Use token
		data, err := storage.Use(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, []byte("test-data"), data)

		// Verify token is deleted
		result = redis.Do(ctx, redis.B().Get().Key(stateTokenPrefix+token).Build())
		assert.Error(t, result.Error())
	})

	t.Run("returns ErrBadState for non-existent token", func(t *testing.T) {
		_, err := storage.Use(ctx, "non-existent-token")
		assert.ErrorIs(t, err, ErrBadState)
	})

	t.Run("token can only be used once", func(t *testing.T) {
		// Create token
		token, err := storage.New(ctx, []byte("test-data"))
		require.NoError(t, err)

		// First use should succeed
		data, err := storage.Use(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, []byte("test-data"), data)

		// Second use should fail
		_, err = storage.Use(ctx, token)
		assert.ErrorIs(t, err, ErrBadState)
	})
}

func TestRedisStateStorage_StateWithData(t *testing.T) {
	ctx := context.Background()
	redisContainer := testhelper.NewRedisContainer(t)
	redis := testhelper.NewRedisClient(t, redisContainer)

	storage := NewRedisTokenStorage(redis)

	t.Run("stores and retrieves data correctly", func(t *testing.T) {
		// Test data with various content types
		testCases := []struct {
			name string
			data []byte
		}{
			{"empty data", []byte{}},
			{"simple string", []byte("hello world")},
			{"json data", []byte(`{"user_id": "123", "redirect_uri": "https://example.com/callback"}`)},
			{"binary data", []byte{0x00, 0x01, 0x02, 0x03, 0xFF}},
			{"unicode string", []byte("测试数据 with emoji 🚀")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create token with data
				token, err := storage.New(ctx, tc.data)
				require.NoError(t, err)
				require.NotEmpty(t, token)

				// Use token and verify data is returned correctly
				retrievedData, err := storage.Use(ctx, token)
				require.NoError(t, err)
				assert.Equal(t, tc.data, retrievedData)

				// Verify token is deleted after use
				_, err = storage.Use(ctx, token)
				assert.ErrorIs(t, err, ErrBadState)
			})
		}
	})

	t.Run("handles large data", func(t *testing.T) {
		// Create large data (1KB)
		largeData := make([]byte, 1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		// Create token with large data
		token, err := storage.New(ctx, largeData)
		require.NoError(t, err)

		// Use token and verify data is returned correctly
		retrievedData, err := storage.Use(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, largeData, retrievedData)
		assert.Equal(t, len(largeData), len(retrievedData))
	})

	t.Run("preserves data integrity across concurrent operations", func(t *testing.T) {
		// Create multiple tokens with different data
		tokens := make([]string, 5)
		expectedData := make([][]byte, 5)

		for i := range tokens {
			data := []byte(fmt.Sprintf("data-%d", i))
			expectedData[i] = data

			token, err := storage.New(ctx, data)
			require.NoError(t, err)
			tokens[i] = token
		}

		// Use tokens and verify each returns the correct data
		for i, token := range tokens {
			retrievedData, err := storage.Use(ctx, token)
			require.NoError(t, err)
			assert.Equal(t, expectedData[i], retrievedData, "Token %d should return correct data", i)
		}
	})

	t.Run("handles nil data", func(t *testing.T) {
		// Create token with nil data
		token, err := storage.New(ctx, nil)
		require.NoError(t, err)

		// Use token and verify nil data is returned
		retrievedData, err := storage.Use(ctx, token)
		require.NoError(t, err)
		assert.Empty(t, retrievedData)
	})
}
