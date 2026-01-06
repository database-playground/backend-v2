// Package deps contains the dependencies for the backend and admin-cli.
package deps

import (
	"context"
	"fmt"
	"log/slog"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/otelprovider"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisotel"
	"go.uber.org/fx"
)

// EntClient creates an ent.Client from a DatabaseConfig.
func EntClient(cfg config.DatabaseConfig) (*ent.Client, error) {
	pgxConfig, err := pgxpool.ParseConfig(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	drv := entsql.OpenDB(dialect.Postgres, db)

	return ent.NewClient(ent.Driver(drv)), nil
}

// RedisClient creates a rueidis.Client from a RedisConfig.
func RedisClient(cfg config.RedisConfig) (rueidis.Client, error) {
	client, err := rueidisotel.NewClient(rueidis.ClientOption{
		InitAddress: []string{
			fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		},
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create redis client: %w", err)
	}

	return client, nil
}

func OTelSDK(lifecycle fx.Lifecycle) {
	shutdown, err := otelprovider.SetupOTelSDK(context.Background())
	if err != nil {
		slog.Error("failed to setup OTel SDK", "error", err)
	}
	lifecycle.Append(fx.StopHook(func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("failed to shutdown OTel SDK", "error", err)
		}
	}))
}
