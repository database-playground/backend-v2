package useraccount

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
)

// DeleteUser deletes a user.
func (c *Context) DeleteUser(ctx context.Context, userID int) error {
	err := c.entClient.User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrUserNotFound
		}

		return err
	}

	return nil
}
