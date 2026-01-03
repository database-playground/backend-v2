package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/redis/rueidis"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := tracer.Start(ctx, "Get",
		trace.WithAttributes(
			attribute.String("auth.token.prefix", redisTokenPrefix),
			attribute.Int64("auth.token.expire_seconds", s.tokenExpire),
		))
	defer span.End()

	tokenInfo, err := s.Peek(ctx, token)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to peek token")
		span.RecordError(err)
		return TokenInfo{}, err
	}

	// extend the expiration time
	span.AddEvent("token.expire.extend")
	replies := s.redis.DoMulti(
		ctx,
		s.redis.B().Expire().Key(redisTokenPrefix+token).Seconds(s.tokenExpire).Build(),
	)
	for _, reply := range replies {
		if reply.Error() != nil {
			span.SetStatus(otelcodes.Error, "Failed to extend token expiration")
			span.RecordError(reply.Error())
			return TokenInfo{}, reply.Error()
		}
	}

	span.SetStatus(otelcodes.Ok, "Token retrieved and expiration extended successfully")
	return tokenInfo, nil
}

func (s *RedisStorage) Peek(ctx context.Context, token string) (TokenInfo, error) {
	ctx, span := tracer.Start(ctx, "Peek",
		trace.WithAttributes(
			attribute.String("auth.token.prefix", redisTokenPrefix),
		))
	defer span.End()

	tokenKey := redisTokenPrefix + token

	span.AddEvent("redis.json.get")
	reply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(tokenKey).Path(".").Build())
	if err := reply.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			span.AddEvent("token.not_found")
			span.SetStatus(otelcodes.Ok, "Token not found")
			return TokenInfo{}, ErrNotFound
		}

		span.SetStatus(otelcodes.Error, "Failed to get token from Redis")
		span.RecordError(err)
		return TokenInfo{}, err
	}

	var tokenInfo TokenInfo
	err := reply.DecodeJSON(&tokenInfo)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to decode token info")
		span.RecordError(err)
		return TokenInfo{}, err
	}

	span.SetAttributes(
		attribute.Int("auth.token.user_id", tokenInfo.UserID),
		attribute.String("auth.token.user_email", tokenInfo.UserEmail),
		attribute.String("auth.token.machine", tokenInfo.Machine),
		attribute.Int("auth.token.scopes.count", len(tokenInfo.Scopes)),
	)
	span.SetStatus(otelcodes.Ok, "Token peeked successfully")
	return tokenInfo, nil
}

func (s *RedisStorage) Create(ctx context.Context, info TokenInfo) (string, error) {
	ctx, span := tracer.Start(ctx, "Create",
		trace.WithAttributes(
			attribute.String("auth.token.prefix", redisTokenPrefix),
			attribute.Int("auth.token.user_id", info.UserID),
			attribute.String("auth.token.user_email", info.UserEmail),
			attribute.String("auth.token.machine", info.Machine),
			attribute.Int("auth.token.scopes.count", len(info.Scopes)),
			attribute.Int64("auth.token.expire_seconds", s.tokenExpire),
		))
	defer span.End()

	span.AddEvent("token.generation")
	token, err := authutil.GenerateToken()
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to generate token")
		span.RecordError(err)
		return "", fmt.Errorf("generate token: %w", err)
	}

	tokenKey := redisTokenPrefix + token

	span.AddEvent("redis.json.set")
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
			span.SetStatus(otelcodes.Error, "Failed to create token in Redis")
			span.RecordError(reply.Error())
			return "", reply.Error()
		}
	}

	span.SetStatus(otelcodes.Ok, "Token created successfully")
	return token, nil
}

func (s *RedisStorage) Delete(ctx context.Context, token string) error {
	ctx, span := tracer.Start(ctx, "Delete",
		trace.WithAttributes(
			attribute.String("auth.token.prefix", redisTokenPrefix),
		))
	defer span.End()

	span.AddEvent("redis.delete")
	deleted, err := s.redis.Do(ctx, s.redis.B().Del().Key(redisTokenPrefix+token).Build()).AsInt64()
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to delete token from Redis")
		span.RecordError(err)
		return err
	}

	if deleted == 0 {
		span.AddEvent("token.not_found")
		span.SetStatus(otelcodes.Ok, "Token not found")
		return ErrNotFound
	}

	span.SetAttributes(attribute.Int64("auth.token.deleted_count", deleted))
	span.SetStatus(otelcodes.Ok, "Token deleted successfully")
	return nil
}

func (s *RedisStorage) DeleteByUser(ctx context.Context, userID int) error {
	ctx, span := tracer.Start(ctx, "DeleteByUser",
		trace.WithAttributes(
			attribute.Int("auth.token.user_id", userID),
			attribute.String("auth.token.prefix", redisTokenPrefix),
		))
	defer span.End()

	var cursor uint64 = 0
	deletedCount := 0

	for {
		span.AddEvent("redis.scan")
		cursorReply := s.redis.Do(ctx, s.redis.B().Scan().Cursor(cursor).Match(redisTokenPrefix+"*").Build())
		if cursorReply.Error() != nil {
			span.SetStatus(otelcodes.Error, "Failed to scan tokens")
			span.RecordError(cursorReply.Error())
			return fmt.Errorf("list tokens: %w", cursorReply.Error())
		}

		scanEntry, err := cursorReply.AsScanEntry()
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to parse scan entry")
			span.RecordError(err)
			return fmt.Errorf("parse token keys: %w", err)
		}

		span.SetAttributes(attribute.Int("redis.scan.elements.count", len(scanEntry.Elements)))

		for _, element := range scanEntry.Elements {
			// get "user" json key
			elementReply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(element).Path("user_id").Build())
			if elementReply.Error() != nil {
				span.SetStatus(otelcodes.Error, "Failed to get token info")
				span.RecordError(elementReply.Error())
				return fmt.Errorf("get token info: %w", elementReply.Error())
			}

			var elementUser int
			err = elementReply.DecodeJSON(&elementUser)
			if err != nil {
				span.SetStatus(otelcodes.Error, "Failed to decode token user ID")
				span.RecordError(err)
				return fmt.Errorf("get token info: %w", err)
			}

			if elementUser != userID {
				continue
			}

			span.AddEvent("token.delete.matched")
			delReply := s.redis.Do(ctx, s.redis.B().Del().Key(element).Build())
			if delReply.Error() != nil {
				span.SetStatus(otelcodes.Error, "Failed to delete matched token")
				span.RecordError(delReply.Error())
				return fmt.Errorf("delete token: %w", delReply.Error())
			}
			deletedCount++
		}

		if scanEntry.Cursor == 0 {
			break
		}

		cursor = scanEntry.Cursor
	}

	span.SetAttributes(attribute.Int("auth.token.deleted_count", deletedCount))
	span.SetStatus(otelcodes.Ok, "Tokens deleted successfully")
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
	ctx, span := tracer.Start(ctx, "GetCurrentTTL",
		trace.WithAttributes(
			attribute.String("auth.token.prefix", redisTokenPrefix),
		))
	defer span.End()

	span.AddEvent("redis.ttl.get")
	reply := s.redis.Do(ctx, s.redis.B().Ttl().Key(redisTokenPrefix+token).Build())
	if reply.Error() != nil {
		span.SetStatus(otelcodes.Error, "Failed to get token TTL")
		span.RecordError(reply.Error())
		return 0, reply.Error()
	}

	ttl, err := reply.AsInt64()
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to decode TTL")
		span.RecordError(err)
		return 0, err
	}

	span.SetAttributes(attribute.Int64("auth.token.ttl_seconds", ttl))
	span.SetStatus(otelcodes.Ok, "Token TTL retrieved successfully")
	return ttl, nil
}

var (
	_ Storage = &RedisStorage{}
	_ Storage = &TestRedisStorage{}
)
