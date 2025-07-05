package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"entgo.io/ent/dialect"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/restapi"
	authservice "github.com/database-playground/backend-v2/restapi/auth"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"
)

// provideConfig loads the environment variables from the .env file and returns a config.Config.
func provideConfig() (config.Config, error) {
	err := godotenv.Load()
	if err != nil {
		slog.Warn("error loading .env file", "error", err)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("error creating config", "error", err)
		return config.Config{}, err
	}

	return cfg, nil
}

// provideEntClient creates an ent.Client.
func provideEntClient() (*ent.Client, error) {
	client, err := ent.Open(dialect.SQLite, "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		slog.Error("error creating ent client", "error", err)
		return nil, err
	}

	return client, nil
}

// provideRedisClient creates a rueidis.Client.
func provideRedisClient(cfg config.Config) (rueidis.Client, error) {
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

// provideAuthMiddleware creates an auth.Middleware that can be injected into gin.
func provideAuthMiddleware(storage auth.Storage) gin.HandlerFunc {
	return auth.Middleware(storage)
}

func provideGqlgenHandler(entClient *ent.Client, storage auth.Storage) *handler.Server {
	srv := handler.New(graph.NewSchema(entClient, storage))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.SetErrorPresenter(graph.NewErrorPresenter())

	return srv
}

func provideAuthService(storage auth.Storage, config config.Config) *authservice.AuthService {
	return authservice.NewAuthService(storage, config)
}

func provideGinEngine(authMiddleware gin.HandlerFunc, gqlgenHandler *handler.Server, services []restapi.Service) *gin.Engine {
	engine := gin.New()

	engine.Use(authMiddleware)
	engine.Use(gin.Recovery())

	engine.GET("/", func(ctx *gin.Context) {
		handler := playground.Handler("GraphQL playground", "/query")
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	})
	engine.POST("/query", func(ctx *gin.Context) {
		gqlgenHandler.ServeHTTP(ctx.Writer, ctx.Request)
	})

	api := engine.Group("/api")
	restapi.Register(api, services...)

	return engine
}

func newGinLifecycle(lifecycle fx.Lifecycle, engine *gin.Engine, cfg config.Config) {
	httpCtx, cancel := context.WithCancel(context.Background())

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
			if err != nil {
				return err
			}

			if httpCtx.Err() != nil {
				listener.Close()
				return nil
			}

			go func() {
				if err := engine.RunListener(listener); err != nil {
					slog.Error("error running gin engine", "error", err)
				}
			}()

			go func() {
				<-httpCtx.Done()
				listener.Close()
			}()

			slog.Info("gin engine started", "address", "http://"+listener.Addr().String())
			return nil
		},
		OnStop: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				cancel()
			}

			return nil
		},
	})
}
