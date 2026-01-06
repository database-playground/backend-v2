package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/deps"
	"github.com/database-playground/backend-v2/internal/metrics"
	"github.com/database-playground/backend-v2/internal/workers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
)

func ExporterConfig() (config.ExporterConfig, error) {
	return config.LoadExporterConfig()
}

func EntClient(cfg config.ExporterConfig) (*ent.Client, error) {
	return deps.EntClient(cfg.Database)
}

func PrometheusMetrics(entClient *ent.Client) prometheus.Gatherer {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),

		metrics.NewEventCollector(entClient),
		metrics.NewSubmissionCollector(entClient),
	)

	return registry
}

func PrometheusHTTPHandler(cfg config.ExporterConfig, gatherer prometheus.Gatherer, lifecycle fx.Lifecycle) {
	httpCtx, cancel := context.WithCancel(context.Background())

	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			http.Handle("GET /metrics", promhttp.HandlerFor(
				gatherer,
				promhttp.HandlerOpts{
					MaxRequestsInFlight:                 100,
					Timeout:                             10 * time.Second,
					EnableOpenMetrics:                   true,
					EnableOpenMetricsTextCreatedSamples: true,
				},
			))

			srv := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Port),
				Handler: nil,
			}

			workers.Global.Go(func() {
				slog.Info("prometheus http handler starting", "address", srv.Addr)
				if err := srv.ListenAndServe(); err != nil {
					if errors.Is(err, http.ErrServerClosed) {
						return
					}

					slog.Error("error starting prometheus http handler", "error", err)
				}
			})

			workers.Global.Go(func() {
				<-httpCtx.Done()

				slog.Info("prometheus http handler shutting down")
				if err := srv.Shutdown(context.Background()); err != nil {
					slog.Error("error shutting down prometheus http handler", "error", err)
				}
			})

			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			workers.Global.Wait()

			return nil
		},
	})
}
