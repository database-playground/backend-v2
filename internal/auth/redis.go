package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/redis/rueidis"
)

// RedisStorage is the storage for authentication token.
type RedisStorage struct {
	redis rueidis.Client
}

const redisTokenPrefix = "auth:token:"

// NewRedisStorage creates a new RedisStorage.
func NewRedisStorage(redis rueidis.Client) *RedisStorage {
	return &RedisStorage{redis: redis}
}

func (s *RedisStorage) Get(ctx context.Context, token string) (TokenInfo, error) {
	tokenKey := redisTokenPrefix + token

	reply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(tokenKey).Path(".").Build())
	if reply.Error() != nil {
		return TokenInfo{}, reply.Error()
	}

	var tokenInfo TokenInfo
	err := reply.DecodeJSON(&tokenInfo)
	if err != nil {
		return TokenInfo{}, err
	}

	return tokenInfo, nil
}

func (s *RedisStorage) Create(ctx context.Context, info TokenInfo) (string, error) {
	tokenBytes := make([]byte, 64)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := base64.StdEncoding.EncodeToString(tokenBytes)

	tokenKey := redisTokenPrefix + token

	tokenInfoBytes, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("marshal token info: %w", err)
	}

	reply := s.redis.Do(ctx, s.redis.B().JsonSet().Key(tokenKey).Path(".").Value(rueidis.BinaryString(tokenInfoBytes)).Build())
	if reply.Error() != nil {
		return "", reply.Error()
	}

	return token, nil
}

func (s *RedisStorage) Delete(ctx context.Context, token string) error {
	reply := s.redis.Do(ctx, s.redis.B().Del().Key(redisTokenPrefix+token).Build())
	if reply.Error() != nil {
		return reply.Error()
	}

	return nil
}

func (s *RedisStorage) DeleteByUser(ctx context.Context, user string) error {
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
			elementReply := s.redis.Do(ctx, s.redis.B().JsonGet().Key(element).Path("user").Build())
			if elementReply.Error() != nil {
				return fmt.Errorf("get token info: %w", elementReply.Error())
			}

			var elementUser string

			elementUserJson, err := elementReply.AsBytes()
			if err != nil {
				return fmt.Errorf("get token info: %w", err)
			}

			fmt.Println(string(elementUserJson))

			err = json.Unmarshal(elementUserJson, &elementUser)
			if err != nil {
				return fmt.Errorf("unmarshal user: %w", err)
			}

			if elementUser != user {
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
