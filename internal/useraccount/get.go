package useraccount

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/user"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("dbplay.useraccount")

func (c *Context) GetUser(ctx context.Context, userID int) (*ent.User, error) {
	ctx, span := tracer.Start(ctx, "GetUser",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	user, err := c.entClient.User.Query().Where(user.ID(userID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Error, "User not found")
			return nil, ErrUserNotFound
		}
		span.SetStatus(otelcodes.Error, "Failed to get user")
		span.RecordError(err)
		return nil, err
	}

	span.SetStatus(otelcodes.Ok, "User retrieved successfully")
	return user, nil
}
