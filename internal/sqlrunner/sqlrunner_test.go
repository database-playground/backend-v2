package sqlrunner_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
)

func newTestSqlRunner(t *testing.T) *sqlrunner.SqlRunner {
	t.Helper()

	baseUri := os.Getenv("SQLRUNNER_BASE_URI")
	if baseUri == "" {
		t.Skip("SQLRUNNER_BASE_URI is not set")
	}

	cfg := config.SqlRunnerConfig{
		URI: baseUri,
	}
	return sqlrunner.NewSqlRunner(cfg)
}

func TestHealthz(t *testing.T) {
	s := newTestSqlRunner(t)
	if !s.IsHealthy(context.Background()) {
		t.Error("Expected IsHealthy to return true, got false")
	}
}

func TestQuery_Success(t *testing.T) {
	s := newTestSqlRunner(t)
	data, err := s.Query(context.Background(), "CREATE TABLE dev(ID int); INSERT INTO dev VALUES(1);", "SELECT * FROM dev;")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(data.Columns) != 1 || data.Columns[0] != "ID" {
		t.Errorf("Expected columns [ID], got %v", data.Columns)
	}
	if len(data.Rows) != 1 || data.Rows[0][0] != "1" {
		t.Errorf("Expected rows [[1]], got %v", data.Rows)
	}
}

func TestQuery_QueryError(t *testing.T) {
	s := newTestSqlRunner(t)
	_, err := s.Query(context.Background(), "CREATE TABLE dev(ID int); INSERT INTO dev VALUES(1);", "SELECT * FROM non_existing_table;")
	if err == nil || err.Error() == "" {
		t.Error("Expected query error, got nil")
	}
	if !errors.Is(err, sqlrunner.ErrQueryError) {
		t.Errorf("Expected QUERY_ERROR, got %v", err)
	}
}

func TestQuery_SchemaError(t *testing.T) {
	s := newTestSqlRunner(t)
	_, err := s.Query(context.Background(), "CREATE TABLE dev(ID int", "SELECT * FROM dev;")
	if err == nil || err.Error() == "" {
		t.Error("Expected schema error, got nil")
	}
	if !errors.Is(err, sqlrunner.ErrSchemaError) {
		t.Errorf("Expected SCHEMA_ERROR, got %v", err)
	}
}
