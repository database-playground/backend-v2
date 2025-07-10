package useraccount

import (
	"context"
	"errors"
	"fmt"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
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
	// check if user already exists
	user, err := c.entClient.User.Query().Where(user.EmailEQ(req.Email)).Only(ctx)
	if err == nil {
		return user, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("check user existence: %w", err)
	}

	unverifiedGroup, err := c.entClient.Group.Query().Where(group.NameEQ(UnverifiedGroupSlug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrIncompleteSetup
		}

		return nil, fmt.Errorf("check unverified group: %w", err)
	}

	user, err = c.entClient.User.Create().
		SetEmail(req.Email).
		SetName(req.Name).
		SetGroup(unverifiedGroup).
		SetAvatar(req.Avatar).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Verify verifies a user by moving them from the unverified group to the new user group.
func (c *Context) Verify(ctx context.Context, userID int) error {
	// Check if this user is in the unverified group
	user, err := c.entClient.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	groupModel, err := user.QueryGroup().Only(ctx)
	if err != nil {
		return fmt.Errorf("get group: %w", err)
	}

	if groupModel.Name != UnverifiedGroupSlug {
		return ErrUserVerified
	}

	newUserGroup, err := c.entClient.Group.Query().Where(group.NameEQ(NewUserGroupSlug)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrIncompleteSetup
		}

		return fmt.Errorf("get new user group: %w", err)
	}

	// update user's group to the new user group
	if _, err := user.Update().SetGroup(newUserGroup).Save(ctx); err != nil {
		return fmt.Errorf("update user group: %w", err)
	}

	return nil
}
