package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph"
	"github.com/database-playground/backend-v2/httpapi"
	authservice "github.com/database-playground/backend-v2/httpapi/auth"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/httputils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/rueidis"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"

	_ "github.com/database-playground/backend-v2/internal/deps/logger"
)

// AuthStorage creates an auth.Storage.
func AuthStorage(redisClient rueidis.Client) auth.Storage {
	return auth.NewRedisStorage(redisClient)
}

// AuthMiddleware creates an auth.Middleware that can be injected into gin.
func AuthMiddleware(storage auth.Storage) Middleware {
	return Middleware{
		Handler: auth.Middleware(storage),
	}
}

// MachineMiddleware creates a machine middleware that can be injected into gin.
func MachineMiddleware() Middleware {
	return Middleware{
		Handler: httputils.MachineMiddleware(),
	}
}

// CorsMiddleware creates a cors middleware that can be injected into gin.
func CorsMiddleware(cfg config.Config) Middleware {
	return Middleware{
		Handler: cors.New(cors.Config{
			AllowOrigins:     cfg.AllowedOrigins,
			AllowMethods:     []string{"GET", "POST", "OPTIONS"},
			AllowHeaders:     []string{"Content-Type", "User-Agent", "Referer"},
			AllowCredentials: true,
		}),
	}
}

// GqlgenHandler creates a gqlgen handler.
func GqlgenHandler(entClient *ent.Client, storage auth.Storage) *handler.Server {
	srv := handler.New(graph.NewSchema(entClient, storage))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(entgql.Transactioner{TxOpener: entClient})
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.SetErrorPresenter(graph.NewErrorPresenter())

	return srv
}

// AuthService creates an auth service.
func AuthService(entClient *ent.Client, storage auth.Storage, config config.Config) httpapi.Service {
	return authservice.NewAuthService(entClient, storage, config)
}

// GinEngine creates a gin engine.
func GinEngine(services []httpapi.Service, middlewares []Middleware, gqlgenHandler *handler.Server, cfg config.Config) *gin.Engine {
	engine := gin.New()

	if err := engine.SetTrustedProxies(cfg.TrustProxies); err != nil {
		slog.Error("error setting trusted proxies", "error", err)
	}

	for _, middleware := range middlewares {
		engine.Use(middleware.Handler)
	}

	engine.Use(gin.Recovery())

	engine.GET("/", func(ctx *gin.Context) {
		handler := playground.Handler("GraphQL playground", "/query")
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	})
	engine.POST("/query", func(ctx *gin.Context) {
		gqlgenHandler.ServeHTTP(ctx.Writer, ctx.Request)
	})

	api := engine.Group("/api")
	httpapi.Register(api, services...)

	return engine
}

// GinLifecycle starts the gin engine.
func GinLifecycle(lifecycle fx.Lifecycle, engine *gin.Engine, cfg config.Config) {
	httpCtx, cancel := context.WithCancel(context.Background())

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			srv := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Port),
				Handler: engine,
			}

			go func() {
				slog.Info("gin engine starting", "address", srv.Addr, "proto", cfg.Server.GetProto())

				if cfg.Server.CertFile != nil && cfg.Server.KeyFile != nil {
					if err := srv.ListenAndServeTLS(*cfg.Server.CertFile, *cfg.Server.KeyFile); err != nil {
						if errors.Is(err, http.ErrServerClosed) {
							return
						}

						slog.Error("error running gin engine with TLS", "error", err)
					}
				} else {
					if err := srv.ListenAndServe(); err != nil {
						if errors.Is(err, http.ErrServerClosed) {
							return
						}

						slog.Error("error running gin engine", "error", err)
					}
				}
			}()

			go func() {
				<-httpCtx.Done()
				if err := srv.Shutdown(context.Background()); err != nil {
					slog.Error("error shutting down gin engine", "error", err)
				}
			}()

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

// Middleware is a middleware that can be injected into gin.
type Middleware struct {
	Handler gin.HandlerFunc
}

// AnnotateMiddleware annotates a middleware function to be injected into gin.
func AnnotateMiddleware(f any) any {
	return fx.Annotate(
		f,
		fx.ResultTags(`group:"middlewares"`),
	)
}

// AnnotateService annotates a service function to be injected into gin.
func AnnotateService(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(httpapi.Service)),
		fx.ResultTags(`group:"services"`),
	)
}
