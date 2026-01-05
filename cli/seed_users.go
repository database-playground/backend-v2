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
	"github.com/samber/lo"
)

// SeedUsers creates users from the provided seed records.
//
// Each record must contain a valid email and an optional group slug.
// If the referenced group does not exist, an error is returned.
// If a user with the same email already exists, it is skipped without error.
//
// SeedUsers returns an error if validation, group lookup, or user creation fails.
func (c *Context) SeedUsers(ctx context.Context, userSeedRecords []UserSeedRecord) error {
	// validate records
	if len(userSeedRecords) == 0 {
		return nil
	}

	for i, record := range userSeedRecords {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("user seed record #%d: %w", i, err)
		}
	}

	// collect unique group names
	uniqueGroupName := lo.Uniq(lo.Map(userSeedRecords, func(record UserSeedRecord, _ int) string {
		return record.GetGroup()
	}))

	// query all required groups in a single batch
	entGroups, err := c.entClient.Group.Query().Where(group.NameIn(uniqueGroupName...)).All(ctx)
	if err != nil {
		return fmt.Errorf("query groups: %w", err)
	}

	groups := make(map[string]*ent.Group, len(entGroups))
	for _, g := range entGroups {
		groups[g.Name] = g
	}

	// ensure all requested groups were found
	for _, name := range uniqueGroupName {
		if _, ok := groups[name]; !ok {
			return fmt.Errorf("group %q not found", name)
		}
	}
	// create users
	for _, record := range userSeedRecords {
		group := groups[record.GetGroup()]

		// check if user already exists
		count, err := c.entClient.User.Query().Where(user.EmailEQ(record.Email)).Count(ctx)
		if err != nil {
			return fmt.Errorf("count users %q: %w", record.Email, err)
		}
		if count > 0 {
			log.Printf("⚠️ User %q already exists, skipping creation", record.Email)
			continue
		}

		newUser, err := c.entClient.User.Create().
			SetEmail(record.GetEmail()).
			SetName(record.GetEmail()). // Use email as name for seeded users
			SetGroup(group).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create user %q: %w", record.Email, err)
		}

		log.Printf("✅ User %q (Group %q, %d) is created", newUser.Email, group.Name, group.ID)
	}

	return nil
}

// UserSeedRecord represents a user seed record.
type UserSeedRecord struct {
	Email string `json:"email"`
	Group string `json:"group"`
}

// GetEmail returns the email of the user seed record.
func (r UserSeedRecord) GetEmail() string {
	return r.Email
}

// GetGroup returns the group of the user seed record.
// If the group slug is omitted, the user will be added to the `student` group.
func (r UserSeedRecord) GetGroup() string {
	if r.Group == "" {
		return useraccount.StudentGroupSlug
	}

	return r.Group
}

// Validate validates the user seed record.
func (r UserSeedRecord) Validate() error {
	if r.Email == "" {
		return errors.New("email is required")
	}

	return nil
}
