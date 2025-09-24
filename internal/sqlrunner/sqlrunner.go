package sqlrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/database-playground/backend-v2/internal/config"
)

type SqlRunner struct {
	client *http.Client
	cfg    config.SqlRunnerConfig
}

func NewSqlRunner(cfg config.SqlRunnerConfig) *SqlRunner {
	return &SqlRunner{
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (s *SqlRunner) Query(ctx context.Context, schema, query string) (DataResponse, error) {
	payload := QueryRequest{
		Schema: schema,
		Query:  query,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return DataResponse{}, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/query", s.cfg.URI), bytes.NewBuffer(body))
	if err != nil {
		return DataResponse{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return DataResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	var respBody QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return DataResponse{}, fmt.Errorf("bad response: %w", err)
	}

	// check if success
	if respBody.ErrorResponse != nil {
		return DataResponse{}, respBody.ErrorResponse
	}

	if respBody.SuccessResponse == nil {
		return DataResponse{}, fmt.Errorf("success response is nil")
	}

	return respBody.Data, nil
}

func (s *SqlRunner) GetDatabaseStructure(ctx context.Context, schema string) (DatabaseStructure, error) {
	// Query SQLite's master table to get all table names
	tablesQuery := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	tablesResp, err := s.Query(ctx, schema, tablesQuery)
	if err != nil {
		return DatabaseStructure{}, fmt.Errorf("failed to query tables: %w", err)
	}

	var tables []DatabaseTable

	// For each table, get its column information
	for _, row := range tablesResp.Rows {
		if len(row) == 0 {
			continue
		}
		tableName := row[0]

		// Use PRAGMA table_info to get column information
		columnsQuery := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
		columnsResp, err := s.Query(ctx, schema, columnsQuery)
		if err != nil {
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

	return DatabaseStructure{
		Tables: tables,
	}, nil
}

func (s *SqlRunner) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/healthz", s.cfg.URI), nil)
	if err != nil {
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	return resp.StatusCode == http.StatusOK
}
