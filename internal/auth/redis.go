package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/redis/rueidis"
)

// RedisStorage is the storage for authentication token.
type RedisStorage struct {
	redis       rueidis.Client
	tokenExpire int64
}

type RedisStorageOption func(*RedisStorage)

func WithTokenExpire(expire time.Duration) RedisStorageOption {
	return func(s *RedisStorage) {
		s.tokenExpire = int64(expire.Seconds())
	}
}

const redisTokenPrefix = "auth:token:"

// NewRedisStorage creates a new RedisStorage.
func NewRedisStorage(redis rueidis.Client, opts ...RedisStorageOption) *TestRedisStorage {
	s := &RedisStorage{redis: redis, tokenExpire: DefaultTokenExpire}
	for _, opt := range opts {
		opt(s)
	}

	return &TestRedisStorage{RedisStorage: s}
}

func (s *RedisStorage) Get(ctx context.Context, token string) (TokenInfo, error) {
	tokenInfo, err := s.Peek(ctx, token)
	if err != nil {
		return TokenInfo{}, err
	}

	// extend the expiration time
	replies := s.redis.DoMulti(
		ctx,
		s.redis.B().Expire().Key(redisTokenPrefix+token).Seconds(s.tokenExpire).Build(),
	)
	for _, reply := range replies {
		if reply.Error() != nil {
			return TokenInfo{}, reply.Error()
		}
	}

	return tokenInfo, nil
}

func (s *RedisStorage) Peek(ctx context.Context, token string) (TokenInfo, error) {
	tokenKey := redisTokenPrefix + token

	reply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(tokenKey).Path(".").Build())
	if err := reply.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return TokenInfo{}, ErrNotFound
		}

		return TokenInfo{}, err
	}

	var tokenInfo TokenInfo
	err := reply.DecodeJSON(&tokenInfo)
	if err != nil {
		return TokenInfo{}, err
	}

	return tokenInfo, nil
}

func (s *RedisStorage) Create(ctx context.Context, info TokenInfo) (string, error) {
	token, err := authutil.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	tokenKey := redisTokenPrefix + token

	replies := s.redis.DoMulti(
		ctx,
		s.redis.B().JsonSet().
			Key(tokenKey).
			Path(".").
			Value(rueidis.JSON(info)).
			Build(),
		s.redis.B().Expire().Key(tokenKey).Seconds(s.tokenExpire).Build(),
	)
	for _, reply := range replies {
		if reply.Error() != nil {
			return "", reply.Error()
		}
	}

	return token, nil
}

func (s *RedisStorage) Delete(ctx context.Context, token string) error {
	deleted, err := s.redis.Do(ctx, s.redis.B().Del().Key(redisTokenPrefix+token).Build()).AsInt64()
	if err != nil {
		return err
	}

	if deleted == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *RedisStorage) DeleteByUser(ctx context.Context, userID int) error {
	var cursor uint64 = 0

	for {
		cursorReply := s.redis.Do(ctx, s.redis.B().Scan().Cursor(cursor).Match(redisTokenPrefix+"*").Build())
		if cursorReply.Error() != nil {
			return fmt.Errorf("list tokens: %w", cursorReply.Error())
		}

		scanEntry, err := cursorReply.AsScanEntry()
		if err != nil {
			return fmt.Errorf("parse token keys: %w", err)
		}

		for _, element := range scanEntry.Elements {
			// get "user" json key
			elementReply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(element).Path("user_id").Build())
			if elementReply.Error() != nil {
				return fmt.Errorf("get token info: %w", elementReply.Error())
			}

			var elementUser int
			err = elementReply.DecodeJSON(&elementUser)
			if err != nil {
				return fmt.Errorf("get token info: %w", err)
			}

			if elementUser != userID {
				continue
			}

			delReply := s.redis.Do(ctx, s.redis.B().Del().Key(element).Build())
			if delReply.Error() != nil {
				return fmt.Errorf("delete token: %w", delReply.Error())
			}
		}

		if scanEntry.Cursor == 0 {
			break
		}

		cursor = scanEntry.Cursor
	}

	return nil
}

// TestRedisStorage is a RedisStorage for testing purpose.
//
// It contains some extra methods for inspecting the token.
// You should never use this in production since it is not API stable.
type TestRedisStorage struct {
	*RedisStorage
}

// GetCurrentTTL returns the current TTL of the token.
//
// This is only for testing purpose.
func (s *TestRedisStorage) GetCurrentTTL(ctx context.Context, token string) (int64, error) {
	reply := s.redis.Do(ctx, s.redis.B().Ttl().Key(redisTokenPrefix+token).Build())
	if reply.Error() != nil {
		return 0, reply.Error()
	}

	ttl, err := reply.AsInt64()
	if err != nil {
		return 0, err
	}

	return ttl, nil
}

var (
	_ Storage = &RedisStorage{}
	_ Storage = &TestRedisStorage{}
)
