package cli

import (
	"context"

	"github.com/database-playground/backend-v2/internal/setup"
)

// Migrate the database to the latest version.
func (c *Context) Migrate(ctx context.Context) error {
	return setup.Migrate(ctx, c.entClient)
}
