package cli

import (
	"context"
	"log"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/scopeset"
)

// SetupResult is the result of the setup process.
type SetupResult struct {
	AdminScopeSet   *ent.ScopeSet
	NewUserScopeSet *ent.ScopeSet
	AdminGroup      *ent.Group
	NewUserGroup    *ent.Group
}

// Setup setups the database playground instance.
// It will create the admin scope set, the new-user scope set, the admin group, and the new-user group.
// It will return the result of the setup process.
func (c *Context) Setup(ctx context.Context) (*SetupResult, error) {
	// migrate first
	if err := c.Migrate(ctx); err != nil {
		return nil, err
	}

	// Check if admin scope set already exists
	adminScopeSet, err := c.entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ("admin")).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if adminScopeSet == nil {
		log.Println("[*] Creating the admin scope set…")
		adminScopeSet, err = c.entClient.ScopeSet.Create().
			SetSlug("admin").
			SetDescription("Administrator").
			SetScopes([]string{"*"}).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Admin scope set already exists, skipping creation")
	}

	// Check if new-user scope set already exists
	newUserScopeSet, err := c.entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ("new-user")).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if newUserScopeSet == nil {
		log.Println("[*] Creating the 'new-user' scope set…")
		newUserScopeSet, err = c.entClient.ScopeSet.Create().
			SetSlug("new-user").
			SetDescription("New users can only read their own user data.").
			SetScopes([]string{"user:read"}).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] New-user scope set already exists, skipping creation")
	}

	// Check if admin group already exists
	adminGroup, err := c.entClient.Group.Query().
		Where(group.NameEQ("admin")).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if adminGroup == nil {
		log.Println("[*] Creating the admin group…")
		adminGroup, err = c.entClient.Group.Create().
			SetName("admin").
			SetDescription("Administrator").
			AddScopeSetIDs(adminScopeSet.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Admin group already exists, skipping creation")
	}

	// Check if new-user group already exists
	newUserGroup, err := c.entClient.Group.Query().
		Where(group.NameEQ("new-user")).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if newUserGroup == nil {
		log.Println("[*] Creating the 'new-user' group…")
		newUserGroup, err = c.entClient.Group.Create().
			SetName("new-user").
			SetDescription("New users that is not yet verified.").
			AddScopeSetIDs(newUserScopeSet.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] New-user group already exists, skipping creation")
	}

	return &SetupResult{
		AdminScopeSet:   adminScopeSet,
		NewUserScopeSet: newUserScopeSet,
		AdminGroup:      adminGroup,
		NewUserGroup:    newUserGroup,
	}, nil
}
