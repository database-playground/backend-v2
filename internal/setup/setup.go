package setup

import (
	"context"
	"log"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/scopeset"
	"github.com/database-playground/backend-v2/internal/useraccount"
)

// SetupResult is the result of the setup process.
type SetupResult struct {
	AdminScopeSet      *ent.ScopeSet
	NewUserScopeSet    *ent.ScopeSet
	AdminGroup         *ent.Group
	NewUserGroup       *ent.Group
	UnverifiedScopeSet *ent.ScopeSet
	UnverifiedGroup    *ent.Group
}

// Migrate migrates the database to the latest version.
func Migrate(ctx context.Context, entClient *ent.Client) error {
	return entClient.Schema.Create(ctx)
}

// Setup setups the database playground instance.
func Setup(ctx context.Context, entClient *ent.Client) (*SetupResult, error) {
	// migrate first
	if err := Migrate(ctx, entClient); err != nil {
		return nil, err
	}

	// Check if admin scope set already exists
	adminScopeSet, err := entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ(useraccount.AdminScopeSetSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if adminScopeSet == nil {
		log.Println("[*] Creating the admin scope set…")
		adminScopeSet, err = entClient.ScopeSet.Create().
			SetSlug(useraccount.AdminScopeSetSlug).
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
	newUserScopeSet, err := entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ(useraccount.NewUserScopeSetSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if newUserScopeSet == nil {
		log.Println("[*] Creating the 'new-user' scope set…")
		newUserScopeSet, err = entClient.ScopeSet.Create().
			SetSlug(useraccount.NewUserScopeSetSlug).
			SetDescription("New users can only read and write their own data.").
			SetScopes([]string{"me:*"}).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] New-user scope set already exists, skipping creation")
	}

	// Check if unverified scope set already exists
	unverifiedScopeSet, err := entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ(useraccount.UnverifiedScopeSetSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}
	if unverifiedScopeSet == nil {
		log.Println("[*] Creating the 'unverified' scope set…")
		unverifiedScopeSet, err = entClient.ScopeSet.Create().
			SetSlug(useraccount.UnverifiedScopeSetSlug).
			SetDescription("Unverified users can only verify their account and read their own initial data.").
			SetScopes([]string{"verification:*", "me:read"}).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Unverified scope set already exists, skipping creation")
	}

	// Check if admin group already exists
	adminGroup, err := entClient.Group.Query().
		Where(group.NameEQ(useraccount.AdminGroupSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if adminGroup == nil {
		log.Println("[*] Creating the admin group…")
		adminGroup, err = entClient.Group.Create().
			SetName(useraccount.AdminGroupSlug).
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
	newUserGroup, err := entClient.Group.Query().
		Where(group.NameEQ(useraccount.NewUserGroupSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if newUserGroup == nil {
		log.Println("[*] Creating the 'new-user' group…")
		newUserGroup, err = entClient.Group.Create().
			SetName(useraccount.NewUserGroupSlug).
			SetDescription("New users that is not yet verified.").
			AddScopeSetIDs(newUserScopeSet.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] New-user group already exists, skipping creation")
	}

	// Check if unverified group already exists
	unverifiedGroup, err := entClient.Group.Query().
		Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if unverifiedGroup == nil {
		log.Println("[*] Creating the 'unverified' group…")
		unverifiedGroup, err = entClient.Group.Create().
			SetName(useraccount.UnverifiedGroupSlug).
			SetDescription("Unverified users that is not yet verified.").
			AddScopeSetIDs(unverifiedScopeSet.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Unverified group already exists, skipping creation")
	}

	return &SetupResult{
		AdminScopeSet:      adminScopeSet,
		NewUserScopeSet:    newUserScopeSet,
		AdminGroup:         adminGroup,
		NewUserGroup:       newUserGroup,
		UnverifiedScopeSet: unverifiedScopeSet,
		UnverifiedGroup:    unverifiedGroup,
	}, nil
}
