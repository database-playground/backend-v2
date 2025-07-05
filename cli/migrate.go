package cli

import "context"

// Migrate the database to the latest version.
func (c *Context) Migrate(ctx context.Context) error {
	return c.entClient.Schema.Create(ctx)
}
