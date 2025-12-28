package sqlrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/database-playground/backend-v2/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("dbplay.sqlrunner")

type SqlRunner struct {
	client *http.Client
	cfg    config.SqlRunnerConfig
}

func NewSqlRunner(cfg config.SqlRunnerConfig) *SqlRunner {
	return &SqlRunner{
		cfg: cfg,
		client: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   60 * time.Second,
		},
	}
}

func (s *SqlRunner) Query(ctx context.Context, schema, query string) (DataResponse, error) {
	ctx, span := tracer.Start(ctx, "Query",
		trace.WithAttributes(
			attribute.String("sqlrunner.schema", schema),
			attribute.String("http.method", http.MethodPost),
			attribute.String("http.url", fmt.Sprintf("%s/query", s.cfg.URI)),
		))
	defer span.End()

	payload := QueryRequest{
		Schema: schema,
		Query:  query,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to marshal payload")
		span.RecordError(err)
		return DataResponse{}, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/query", s.cfg.URI), bytes.NewBuffer(body))
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create request")
		span.RecordError(err)
		return DataResponse{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to send request")
		span.RecordError(err)
		return DataResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	var respBody QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		span.SetStatus(otelcodes.Error, "Failed to decode response")
		span.RecordError(err)
		return DataResponse{}, fmt.Errorf("bad response: %w", err)
	}

	// check if success
	if respBody.ErrorResponse != nil {
		span.SetStatus(otelcodes.Error, "Query execution failed")
		span.RecordError(respBody.ErrorResponse)
		return DataResponse{}, respBody.ErrorResponse
	}

	if respBody.SuccessResponse == nil {
		span.SetStatus(otelcodes.Error, "Success response is nil")
		return DataResponse{}, fmt.Errorf("success response is nil")
	}

	span.SetAttributes(
		attribute.Int("sqlrunner.rows_count", len(respBody.Data.Rows)),
		attribute.Int("sqlrunner.columns_count", len(respBody.Data.Columns)),
	)

	span.SetStatus(otelcodes.Ok, "Query executed successfully")
	return respBody.Data, nil
}

func (s *SqlRunner) GetDatabaseStructure(ctx context.Context, schema string) (DatabaseStructure, error) {
	ctx, span := tracer.Start(ctx, "GetDatabaseStructure",
		trace.WithAttributes(
			attribute.String("sqlrunner.schema", schema),
		))
	defer span.End()

	// Query SQLite's master table to get all table names
	span.AddEvent("database.tables.querying")
	tablesQuery := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	tablesResp, err := s.Query(ctx, schema, tablesQuery)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to query tables")
		span.RecordError(err)
		return DatabaseStructure{}, fmt.Errorf("failed to query tables: %w", err)
	}

	var tables []DatabaseTable

	// For each table, get its column information
	span.AddEvent("database.columns.processing")
	for _, row := range tablesResp.Rows {
		if len(row) == 0 {
			continue
		}
		tableName := row[0]

		// Use PRAGMA table_info to get column information
		columnsQuery := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
		columnsResp, err := s.Query(ctx, schema, columnsQuery)
		if err != nil {
			span.SetStatus(otelcodes.Error, fmt.Sprintf("Failed to query columns for table %s", tableName))
			span.RecordError(err)
			return DatabaseStructure{}, fmt.Errorf("query columns for table %s: %w", tableName, err)
		}

		var columns []string
		// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
		// We only need the name (index 1)
		for _, columnRow := range columnsResp.Rows {
			if len(columnRow) > 1 {
				columns = append(columns, columnRow[1])
			}
		}

		tables = append(tables, DatabaseTable{
			Name:    tableName,
			Columns: columns,
		})
	}

	span.SetAttributes(
		attribute.Int("database.tables_count", len(tables)),
	)

	span.SetStatus(otelcodes.Ok, "Database structure retrieved successfully")
	return DatabaseStructure{
		Tables: tables,
	}, nil
}

func (s *SqlRunner) IsHealthy(ctx context.Context) bool {
	ctx, span := tracer.Start(ctx, "IsHealthy",
		trace.WithAttributes(
			attribute.String("http.method", http.MethodGet),
			attribute.String("http.url", fmt.Sprintf("%s/healthz", s.cfg.URI)),
		))
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/healthz", s.cfg.URI), nil)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create health check request")
		span.RecordError(err)
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to send health check request")
		span.RecordError(err)
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	isHealthy := resp.StatusCode == http.StatusOK
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.Bool("health_check.healthy", isHealthy),
	)

	if isHealthy {
		span.SetStatus(otelcodes.Ok, "Service is healthy")
	} else {
		span.SetStatus(otelcodes.Error, "Service is unhealthy")
	}

	return isHealthy
}
