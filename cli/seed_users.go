package cli

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/internal/useraccount"
)

func (c *Context) SeedUsers(ctx context.Context, userSeedRecords []UserSeedRecord) error {
	// validate records
	for i, record := range userSeedRecords {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("user seed record #%d: %w", i, err)
		}
	}

	// query groups for the records
	groups := make(map[string]*ent.Group)
	for _, record := range userSeedRecords {
		group, err := c.entClient.Group.Query().Where(group.NameEQ(record.GetGroup())).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("group %q not found", record.GetGroup())
			}

			return fmt.Errorf("query group %q: %w", record.GetGroup(), err)
		}
		groups[record.GetGroup()] = group
	}

	// create users
	for _, record := range userSeedRecords {
		group, ok := groups[record.GetGroup()]
		if !ok {
			return fmt.Errorf("user record %q: group %q not found", record.GetEmail(), record.GetGroup())
		}

		// check if user already exists
		user, err := c.entClient.User.Query().Where(user.EmailEQ(record.Email)).First(ctx)
		if err == nil {
			log.Printf("⚠️ User %q already exists, skipping creation", record.Email)
			continue
		}

		user, err = c.entClient.User.Create().
			SetEmail(record.GetEmail()).
			SetName(record.GetEmail()). // Use email as name for seeded users
			SetGroup(group).
			Save(ctx)
		if err != nil {
			return err
		}

		log.Printf("✅ User %q (Group %q, %d) is created", user.Email, group.Name, group.ID)
	}

	return nil
}

type UserSeedRecord struct {
	Email string `json:"email"`
	Group string `json:"group"`
}

func (r UserSeedRecord) GetEmail() string {
	return r.Email
}

func (r UserSeedRecord) GetGroup() string {
	if r.Group == "" {
		return useraccount.StudentGroupSlug
	}

	return r.Group
}

func (r UserSeedRecord) Validate() error {
	if r.Email == "" {
		return errors.New("email is required")
	}

	return nil
}
