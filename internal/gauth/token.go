package gauth

import (
	"context"
	"errors"
	"time"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/redis/rueidis"
)

// ErrBadState is returned when the state token is not found.
var ErrBadState = errors.New("bad state")

type StateStorage interface {
	// New creates a new state token.
	//
	// The token is valid for 10 minutes.
	New(ctx context.Context) (string, error)

	// Use uses the state token.
	//
	// If the state token is not found, it returns ErrBadState.
	// After the token is used, it will be deleted.
	Use(ctx context.Context, token string) error
}

const stateTokenPrefix = "gauth:state:"
const stateTokenExpire = 10 * time.Minute

// RedisStateStorage is a state storage that uses Redis.
type RedisStateStorage struct {
	redis rueidis.Client
}

// NewRedisTokenStorage creates a new RedisTokenStorage.
func NewRedisTokenStorage(redis rueidis.Client) *RedisStateStorage {
	return &RedisStateStorage{redis: redis}
}

// New creates a new state token.
func (s *RedisStateStorage) New(ctx context.Context) (string, error) {
	token, err := authutil.GenerateToken()
	if err != nil {
		return "", err
	}

	if err := s.redis.Do(ctx, s.redis.B().Set().Key(stateTokenPrefix+token).Value(token).Ex(stateTokenExpire).Build()).Error(); err != nil {
		return "", err
	}

	return token, nil
}

// Use uses the state token.
//
// If the state token is not found, it returns ErrBadState.
// After the token is used, it will be deleted.
func (s *RedisStateStorage) Use(ctx context.Context, token string) error {
	result := s.redis.Do(ctx, s.redis.B().Del().Key(stateTokenPrefix+token).Build())
	if err := result.Error(); err != nil {
		return err
	}

	// Del returns the number of keys deleted
	deleted, err := result.AsInt64()
	if err != nil {
		return err
	}

	if deleted == 0 {
		return ErrBadState
	}

	return nil
}

// TestRedisStateStorage is the RedisTokenStorage with some test utilities.
type TestRedisStateStorage struct {
	*RedisStateStorage
}

func (s *TestRedisStateStorage) GetCurrentTTL(ctx context.Context, token string) (int64, error) {
	reply, err := s.redis.Do(ctx, s.redis.B().Ttl().Key(stateTokenPrefix+token).Build()).AsInt64()
	if err != nil {
		return 0, err
	}

	return reply, nil
}
