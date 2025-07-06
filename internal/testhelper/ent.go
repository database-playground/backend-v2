package testhelper

import (
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/enttest"
)

// NewEntSqliteClient creates a new in-memory Ent SQLite client for testing.
func NewEntSqliteClient(t *testing.T) *ent.Client {
	t.Helper()

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Failed to close client: %v", err)
		}
	})

	return client
}
