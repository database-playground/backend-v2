package useraccount

import (
	"context"
	"errors"
	"fmt"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type UserRegisterRequest struct {
	Name   string
	Email  string
	Avatar string
}

var (
	ErrIncompleteSetup = errors.New("setup not completed")
	ErrUserVerified    = errors.New("user already verified")
)

func (c *Context) GetOrRegister(ctx context.Context, req UserRegisterRequest) (*ent.User, error) {
	ctx, span := tracer.Start(ctx, "GetOrRegister",
		trace.WithAttributes(
			attribute.String("user.email", req.Email),
		))
	defer span.End()

	span.AddEvent("user.existence.check")
	// check if user already exists
	user, err := c.entClient.User.Query().Where(user.EmailEQ(req.Email)).Only(ctx)
	if err == nil {
		// update name and avatar to match the OAuth user info
		span.AddEvent("user.update.started")
		user, err = user.Update().SetName(req.Name).SetAvatar(req.Avatar).Save(ctx)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to update user")
			span.RecordError(err)
			return nil, fmt.Errorf("update user: %w", err)
		}

		span.SetStatus(otelcodes.Ok, "User updated successfully")
		return user, nil
	}
	if !ent.IsNotFound(err) {
		span.SetStatus(otelcodes.Error, "Failed to check user existence")
		return nil, fmt.Errorf("check user existence: %w", err)
	}

	span.AddEvent("unverified_group.fetching")
	unverifiedGroup, err := c.entClient.Group.Query().Where(group.NameEQ(UnverifiedGroupSlug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Error, "Setup not completed")
			return nil, ErrIncompleteSetup
		}

		span.SetStatus(otelcodes.Error, "Failed to check unverified group")
		span.RecordError(err)
		return nil, fmt.Errorf("check unverified group: %w", err)
	}

	span.AddEvent("user.create.started")
	user, err = c.entClient.User.Create().
		SetEmail(req.Email).
		SetName(req.Name).
		SetGroup(unverifiedGroup).
		SetAvatar(req.Avatar).
		Save(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create user")
		span.RecordError(err)
		return nil, fmt.Errorf("create user: %w", err)
	}

	span.SetStatus(otelcodes.Ok, "User created successfully")
	return user, nil
}

// Verify verifies a user by moving them from the unverified group to the new user group.
func (c *Context) Verify(ctx context.Context, userID int) error {
	ctx, span := tracer.Start(ctx, "Verify",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	span.AddEvent("user.fetching")
	// Check if this user is in the unverified group
	user, err := c.entClient.User.Get(ctx, userID)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get user")
		span.RecordError(err)
		return fmt.Errorf("get user: %w", err)
	}

	span.AddEvent("user.group.fetching")
	groupModel, err := user.QueryGroup().Only(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get group")
		span.RecordError(err)
		return fmt.Errorf("get group: %w", err)
	}

	if groupModel.Name != UnverifiedGroupSlug {
		span.SetStatus(otelcodes.Error, "User already verified")
		return ErrUserVerified
	}

	span.AddEvent("student_group.fetching")
	studentGroup, err := c.entClient.Group.Query().Where(group.NameEQ(StudentGroupSlug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Error, "Setup not completed")
			return ErrIncompleteSetup
		}

		span.SetStatus(otelcodes.Error, "Failed to get new user group")
		span.RecordError(err)
		return fmt.Errorf("get new user group: %w", err)
	}

	span.AddEvent("user.group.update.started")
	// update user's group to the new user group
	if _, err := user.Update().SetGroup(studentGroup).Save(ctx); err != nil {
		span.SetStatus(otelcodes.Error, "Failed to update user group")
		span.RecordError(err)
		return fmt.Errorf("update user group: %w", err)
	}

	span.SetStatus(otelcodes.Ok, "User verified successfully")
	return nil
}
