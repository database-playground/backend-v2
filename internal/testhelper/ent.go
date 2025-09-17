package testhelper

import (
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/enttest"
	"github.com/database-playground/backend-v2/internal/workers"
)

// NewEntSqliteClient creates a new in-memory Ent SQLite client for testing.
func NewEntSqliteClient(t *testing.T) *ent.Client {
	t.Helper()

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")

	t.Cleanup(func() {
		// must wait the workers to finish
		workers.Global.Wait()

		if err := client.Close(); err != nil {
			t.Fatalf("Failed to close client: %v", err)
		}
	})

	return client
}
