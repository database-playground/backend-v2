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
	AdminGroup         *ent.Group
	StudentScopeSet    *ent.ScopeSet
	StudentGroup       *ent.Group
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
	studentScopeSet, err := entClient.ScopeSet.Query().
		Where(scopeset.SlugEQ(useraccount.StudentScopeSetSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if studentScopeSet == nil {
		log.Println("[*] Creating the 'student' scope set…")
		studentScopeSet, err = entClient.ScopeSet.Create().
			SetSlug(useraccount.StudentScopeSetSlug).
			SetDescription("The necessary permissions to use the main app.").
			SetScopes([]string{"me:*", "question:read", "database:read", "ai"}).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Student scope set already exists, skipping creation")
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
			SetDescription("Unverified users can only read their own initial data, and must be manually verified by an administrator.").
			SetScopes([]string{"me:read"}).
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

	// Check if student group already exists
	studentGroup, err := entClient.Group.Query().
		Where(group.NameEQ(useraccount.StudentGroupSlug)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if studentGroup == nil {
		log.Println("[*] Creating the 'student' group…")
		studentGroup, err = entClient.Group.Create().
			SetName(useraccount.StudentGroupSlug).
			SetDescription("Student").
			AddScopeSetIDs(studentScopeSet.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("[*] Student group already exists, skipping creation")
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
		AdminGroup:         adminGroup,
		StudentScopeSet:    studentScopeSet,
		StudentGroup:       studentGroup,
		UnverifiedScopeSet: unverifiedScopeSet,
		UnverifiedGroup:    unverifiedGroup,
	}, nil
}
