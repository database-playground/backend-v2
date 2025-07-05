package cli

import (
	"context"
	"fmt"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
)

func (c *Context) PromoteAdmin(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	user, err := c.entClient.User.Query().Where(user.Email(email)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user with email %q not found", email)
		}

		return err
	}

	// check if we have an admin group
	group, err := c.entClient.Group.Query().Where(group.Name("admin")).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("admin group not found; run \"setup\" first")
		}
	}

	// add the admin group to the user
	err = c.entClient.User.UpdateOne(user).SetGroup(group).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
