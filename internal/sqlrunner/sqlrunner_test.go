package sqlrunner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/internal/testhelper"
)

func TestHealthz(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)
	if !s.IsHealthy(context.Background()) {
		t.Error("Expected IsHealthy to return true, got false")
	}
}

func TestQuery_Success(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)
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
	s := testhelper.NewSQLRunnerClient(t)
	_, err := s.Query(context.Background(), "CREATE TABLE dev(ID int); INSERT INTO dev VALUES(1);", "SELECT * FROM non_existing_table;")
	if err == nil || err.Error() == "" {
		t.Error("Expected query error, got nil")
	}

	var errResp *sqlrunner.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Errorf("Expected ErrorResponse, got %v", err)
	}
	if errResp.Code != sqlrunner.ErrorCodeQueryError {
		t.Errorf("Expected QUERY_ERROR, got %v", errResp.Code)
	}
}

func TestQuery_SchemaError(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)
	_, err := s.Query(context.Background(), "CREATE TABLE dev(ID int", "SELECT * FROM dev;")
	if err == nil || err.Error() == "" {
		t.Error("Expected schema error, got nil")
	}

	var errResp *sqlrunner.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Errorf("Expected ErrorResponse, got %v", err)
	}
	if errResp.Code != sqlrunner.ErrorCodeSchemaError {
		t.Errorf("Expected SCHEMA_ERROR, got %v", errResp.Code)
	}
}

func TestGetDatabaseStructure_Success(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)

	// Create a schema with multiple tables and columns
	schema := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT,
			user_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
	`

	structure, err := s.GetDatabaseStructure(context.Background(), schema)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify we have the expected number of tables
	if len(structure.Tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(structure.Tables))
	}

	// Helper function to find a table by name
	findTable := func(name string) *sqlrunner.DatabaseTable {
		for _, table := range structure.Tables {
			if table.Name == name {
				return &table
			}
		}
		return nil
	}

	// Verify users table
	usersTable := findTable("users")
	if usersTable == nil {
		t.Error("Expected to find 'users' table")
	} else {
		expectedColumns := []string{"id", "name", "email"}
		if len(usersTable.Columns) != len(expectedColumns) {
			t.Errorf("Expected %d columns in users table, got %d", len(expectedColumns), len(usersTable.Columns))
		}
		for i, expected := range expectedColumns {
			if i >= len(usersTable.Columns) || usersTable.Columns[i] != expected {
				t.Errorf("Expected column %d to be '%s', got '%s'", i, expected, usersTable.Columns[i])
			}
		}
	}

	// Verify posts table
	postsTable := findTable("posts")
	if postsTable == nil {
		t.Error("Expected to find 'posts' table")
	} else {
		expectedColumns := []string{"id", "title", "content", "user_id", "created_at"}
		if len(postsTable.Columns) != len(expectedColumns) {
			t.Errorf("Expected %d columns in posts table, got %d", len(expectedColumns), len(postsTable.Columns))
		}
	}

	// Verify categories table
	categoriesTable := findTable("categories")
	if categoriesTable == nil {
		t.Error("Expected to find 'categories' table")
	} else {
		expectedColumns := []string{"id", "name"}
		if len(categoriesTable.Columns) != len(expectedColumns) {
			t.Errorf("Expected %d columns in categories table, got %d", len(expectedColumns), len(categoriesTable.Columns))
		}
	}
}

func TestGetDatabaseStructure_EmptyDatabase(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)

	// Schema that doesn't create any tables - just a comment
	schema := "-- Empty database with no tables"

	structure, err := s.GetDatabaseStructure(context.Background(), schema)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should have no tables
	if len(structure.Tables) != 0 {
		t.Errorf("Expected 0 tables in empty database, got %d", len(structure.Tables))
	}
}

func TestGetDatabaseStructure_ErrorHandling(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)

	// Create a schema with syntax error that should fail
	schema := "CREATE TABLE invalid_syntax (id int"

	_, err := s.GetDatabaseStructure(context.Background(), schema)
	if err == nil {
		t.Error("Expected error for invalid schema, got nil")
	}

	// Should be a schema error since the schema creation fails
	var errResp *sqlrunner.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Errorf("Expected ErrorResponse, got %v", err)
	}
	if errResp.Code != sqlrunner.ErrorCodeSchemaError {
		t.Errorf("Expected SCHEMA_ERROR, got %v", errResp.Code)
	}
}

func TestGetDatabaseStructure_WithViews(t *testing.T) {
	s := testhelper.NewSQLRunnerClient(t)

	// Create schema with both tables and views
	schema := `
		CREATE TABLE products (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			price DECIMAL(10,2)
		);
		CREATE VIEW expensive_products AS 
		SELECT * FROM products WHERE price > 100;
	`

	structure, err := s.GetDatabaseStructure(context.Background(), schema)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should only include tables, not views (since we filter by type='table')
	if len(structure.Tables) != 1 {
		t.Errorf("Expected 1 table (views should be excluded), got %d", len(structure.Tables))
	}

	if structure.Tables[0].Name != "products" {
		t.Errorf("Expected table name 'products', got '%s'", structure.Tables[0].Name)
	}
}
