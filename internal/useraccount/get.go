package useraccount

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/user"
)

func (c *Context) GetUser(ctx context.Context, userID int) (*ent.User, error) {
	user, err := c.entClient.User.Query().Where(user.ID(userID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}
