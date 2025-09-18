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
