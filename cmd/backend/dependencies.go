package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql"
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
	"github.com/database-playground/backend-v2/internal/deps"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/graphql/apq"
	"github.com/database-playground/backend-v2/internal/httputils"
	"github.com/database-playground/backend-v2/internal/ranking"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/internal/submission"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/database-playground/backend-v2/internal/workers"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/posthog/posthog-go"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/rueidis"
	sloggin "github.com/samber/slog-gin"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"

	"github.com/Depado/ginprom"
	_ "github.com/database-playground/backend-v2/internal/deps/logger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
)

// BackendConfig loads the environment variables from the .env file and returns a config.BackendConfig.
func BackendConfig() (config.BackendConfig, error) {
	return config.LoadBackendConfig()
}

// EntClient creates an ent.Client.
func EntClient(cfg config.BackendConfig) (*ent.Client, error) {
	return deps.EntClient(cfg.Database)
}

// RedisClient creates a rueidis.Client.
func RedisClient(cfg config.BackendConfig) (rueidis.Client, error) {
	return deps.RedisClient(cfg.Redis)
}

// AuthStorage creates an auth.Storage.
func AuthStorage(redisClient rueidis.Client) auth.Storage {
	return auth.NewRedisStorage(redisClient)
}

func SqlRunner(cfg config.BackendConfig) *sqlrunner.SqlRunner {
	return sqlrunner.NewSqlRunner(cfg.SqlRunner)
}

func ApqCache(redisClient rueidis.Client) graphql.Cache[string] {
	return apq.NewCache(redisClient, 24*time.Hour)
}

func PostHogClient(lifecycle fx.Lifecycle, cfg config.BackendConfig) (posthog.Client, error) {
	if cfg.PostHog.APIKey == nil || cfg.PostHog.Host == nil {
		slog.Warn("PostHog client is not initialized, because you did not configure a PostHog API key and a host.")
		return nil, nil
	}

	client, err := posthog.NewWithConfig(
		*cfg.PostHog.APIKey,
		posthog.Config{
			Endpoint: *cfg.PostHog.Host,
		},
	)
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.StopHook(func() {
		if err := client.Close(); err != nil {
			slog.Info("failed to close PostHog client", "error", err)
		}
	}))

	return client, nil
}

// GqlgenHandler creates a gqlgen handler.
func GqlgenHandler(
	entClient *ent.Client,
	storage auth.Storage,
	sqlrunner *sqlrunner.SqlRunner,
	useraccount *useraccount.Context,
	eventService *events.EventService,
	submissionService *submission.SubmissionService,
	rankingService *ranking.Service,
	apqCache graphql.Cache[string],
) *handler.Server {
	srv := handler.New(graph.NewSchema(entClient, storage, sqlrunner, useraccount, eventService, submissionService, rankingService))

	srv.Use(otelgqlgen.Middleware())
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(entgql.Transactioner{TxOpener: entClient})
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: apqCache,
	})

	srv.SetErrorPresenter(graph.NewErrorPresenter())

	return srv
}

// UserAccountContext creates a useraccount.Context.
func UserAccountContext(entClient *ent.Client, storage auth.Storage, eventService *events.EventService) *useraccount.Context {
	return useraccount.NewContext(entClient, storage, eventService)
}

// EventService creates an events.EventService.
func EventService(entClient *ent.Client, posthogClient posthog.Client) *events.EventService {
	return events.NewEventService(entClient, posthogClient)
}

// SubmissionService creates a submission.SubmissionService.
func SubmissionService(entClient *ent.Client, eventService *events.EventService, sqlrunner *sqlrunner.SqlRunner) *submission.SubmissionService {
	return submission.NewSubmissionService(entClient, eventService, sqlrunner)
}

// RankingService creates a ranking.Service.
func RankingService(entClient *ent.Client) *ranking.Service {
	return ranking.NewService(entClient)
}

// AuthService creates an auth service.
func AuthService(entClient *ent.Client, storage auth.Storage, config config.BackendConfig, useraccount *useraccount.Context) httpapi.Service {
	return authservice.NewAuthService(entClient, storage, config, useraccount)
}

// GinEngine creates a gin engine.
func GinEngine(
	services []httpapi.Service,
	authStorage auth.Storage,
	gqlgenHandler *handler.Server,
	cfg config.BackendConfig,
) *gin.Engine {
	engine := gin.New()

	if err := engine.SetTrustedProxies(cfg.TrustProxies); err != nil {
		slog.Error("error setting trusted proxies", "error", err)
	}

	ginprom := ginprom.New(
		ginprom.Engine(engine),
		ginprom.Path("/metrics"),
	)

	engine.Use(gin.Recovery())
	engine.Use(gin.ErrorLogger())
	engine.Use(ginprom.Instrument())
	engine.Use(otelgin.Middleware("dbplay.backend"))
	engine.Use(httputils.MachineMiddleware())
	engine.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "User-Agent", "Referer", "Authorization"},
		MaxAge:       24 * time.Hour,
	}))
	engine.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{
		WithSpanID:    true,
		WithTraceID:   true,
		WithUserAgent: true,
	}))

	// Add a middleware to add the trace ID to the response header
	engine.Use(func(c *gin.Context) {
		traceID := trace.SpanContextFromContext(c.Request.Context()).TraceID().String()
		c.Header("X-Trace-ID", traceID)
		c.Next()
	})

	router := engine.Group("/")
	router.Use(auth.Middleware(authStorage))
	router.GET("/", func(ctx *gin.Context) {
		handler := playground.Handler("GraphQL playground", "/query")
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	})
	router.POST("/query", func(ctx *gin.Context) {
		gqlgenHandler.ServeHTTP(ctx.Writer, ctx.Request)
	})

	api := engine.Group("/api")
	httpapi.Register(api, services...)

	return engine
}

// GinLifecycle starts the gin engine.
func GinLifecycle(lifecycle fx.Lifecycle, engine *gin.Engine, cfg config.BackendConfig) {
	httpCtx, cancel := context.WithCancel(context.Background())

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			srv := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Port),
				Handler: engine,
			}

			workers.Global.Go(func() {
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
			})

			workers.Global.Go(func() {
				<-httpCtx.Done()
				if err := srv.Shutdown(context.Background()); err != nil {
					slog.Error("error shutting down gin engine", "error", err)
				}
			})

			return nil
		},
		OnStop: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				cancel()
			}

			// Wait for all workers to finish
			slog.Info("waiting for workers to finish")
			workers.Global.Wait()

			return nil
		},
	})
}

// AnnotateService annotates a service function to be injected into gin.
func AnnotateService(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(httpapi.Service)),
		fx.ResultTags(`group:"services"`),
	)
}
