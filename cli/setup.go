package cli

import (
	"context"

	"github.com/database-playground/backend-v2/internal/setup"
)

// Setup setups the database playground instance.
func (c *Context) Setup(ctx context.Context) (*setup.SetupResult, error) {
	return setup.Setup(ctx, c.entClient)
}
