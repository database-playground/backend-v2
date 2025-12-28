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
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// Config loads the environment variables from the .env file and returns a config.Config.
func Config() (config.Config, error) {
	err := godotenv.Load()
	if err != nil {
		slog.Warn("error loading .env file", "error", err)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("error creating config", "error", err)
		return config.Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("error validating config", "error", err)
		return config.Config{}, err
	}

	return cfg, nil
}

// EntClient creates an ent.Client.
func EntClient(cfg config.Config) (*ent.Client, error) {
	pgxConfig, err := pgxpool.ParseConfig(cfg.Database.URI)
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

// RedisClient creates a rueidis.Client.
func RedisClient(cfg config.Config) (rueidis.Client, error) {
	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{
			fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		},
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
	})
	if err != nil {
		slog.Error("error creating redis client", "error", err)
		return nil, err
	}

	return client, nil
}

var FxCommonModule = fx.Module("common",
	fx.Provide(Config),
	fx.Provide(EntClient),
	fx.Provide(RedisClient),
)

var FxSlogOption = fx.Options(
	fx.WithLogger(func() fxevent.Logger {
		return &fxevent.SlogLogger{Logger: slog.Default()}
	}),
)
