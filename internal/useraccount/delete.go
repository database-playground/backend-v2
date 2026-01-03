package useraccount

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DeleteUser deletes a user.
func (c *Context) DeleteUser(ctx context.Context, userID int) error {
	ctx, span := tracer.Start(ctx, "DeleteUser",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	err := c.entClient.User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Error, "User not found")
			return ErrUserNotFound
		}

		span.SetStatus(otelcodes.Error, "Failed to delete user")
		span.RecordError(err)
		return err
	}

	span.SetStatus(otelcodes.Ok, "User deleted successfully")
	return nil
}
