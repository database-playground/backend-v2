// Package apq implements Apollo Client's Automatic Persisted Queries.
// https://gqlgen.com/reference/apq/
package apq

import (
	"context"
	"log/slog"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/redis/rueidis"
)

type Cache struct {
	client rueidis.Client
	ttl    time.Duration
}

const redisApqPrefix = "apq:"

func NewCache(client rueidis.Client, ttl time.Duration) *Cache {
	return &Cache{client: client, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, query string) (string, bool) {
	reply, err := c.client.Do(ctx, c.client.B().Get().Key(redisApqPrefix+query).Build()).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return "", false
		}

		slog.Warn("error getting apq from cache", "error", err, "query", query)
		return "", false
	}

	return reply, true
}

func (c *Cache) Add(ctx context.Context, query string, value string) {
	err := c.client.Do(ctx, c.client.B().Set().Key(redisApqPrefix+query).Value(value).Ex(c.ttl).Build()).Error()
	if err != nil {
		slog.Warn("error adding apq to cache", "error", err, "query", query)
	}
}

var _ graphql.Cache[string] = &Cache{}
