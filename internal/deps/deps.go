// Package deps contains the dependencies for the backend and admin-cli.
package deps

import (
	"fmt"
	"log/slog"

	"entgo.io/ent/dialect"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"go.uber.org/fx"
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
func EntClient() (*ent.Client, error) {
	client, err := ent.Open(dialect.SQLite, "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		slog.Error("error creating ent client", "error", err)
		return nil, err
	}

	return client, nil
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

// AuthStorage creates an auth.Storage.
func AuthStorage(redisClient rueidis.Client) auth.Storage {
	return auth.NewRedisStorage(redisClient)
}

var FxCommonModule = fx.Module("common",
	fx.Provide(Config),
	fx.Provide(EntClient),
	fx.Provide(RedisClient),
	fx.Provide(AuthStorage),
)
