package setup_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/scopeset"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"

	_ "github.com/mattn/go-sqlite3"
)

func TestSetup(t *testing.T) {
	t.Run("should create all required entities on first run", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()

		// Run setup
		result, err := setup.Setup(ctx, entClient)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify admin scope set was created
		if result.AdminScopeSet == nil {
			t.Fatal("AdminScopeSet should not be nil")
		}
		if result.AdminScopeSet.Slug != useraccount.AdminScopeSetSlug {
			t.Errorf("Expected admin scope set slug to be 'admin', got %s", result.AdminScopeSet.Slug)
		}
		if result.AdminScopeSet.Description != "Administrator" {
			t.Errorf("Expected admin scope set description to be 'Administrator', got %s", result.AdminScopeSet.Description)
		}
		if len(result.AdminScopeSet.Scopes) != 1 || result.AdminScopeSet.Scopes[0] != "*" {
			t.Errorf("Expected admin scope set scopes to be ['*'], got %v", result.AdminScopeSet.Scopes)
		}

		// Verify new-user scope set was created
		if result.NewUserScopeSet == nil {
			t.Fatal("NewUserScopeSet should not be nil")
		}
		if result.NewUserScopeSet.Slug != useraccount.NewUserScopeSetSlug {
			t.Errorf("Expected new-user scope set slug to be 'new-user', got %s", result.NewUserScopeSet.Slug)
		}
		if result.NewUserScopeSet.Description != "New users can only read and write their own data." {
			t.Errorf("Expected new-user scope set description to be 'New users can only read and write their own data.', got %s", result.NewUserScopeSet.Description)
		}
		if len(result.NewUserScopeSet.Scopes) != 1 || result.NewUserScopeSet.Scopes[0] != "me:*" {
			t.Errorf("Expected new-user scope set scopes to be ['me:*'], got %v", result.NewUserScopeSet.Scopes)
		}

		// Verify unverified scope set was created
		if result.UnverifiedScopeSet == nil {
			t.Fatal("UnverifiedScopeSet should not be nil")
		}
		if result.UnverifiedScopeSet.Slug != useraccount.UnverifiedScopeSetSlug {
			t.Errorf("Expected unverified scope set slug to be 'unverified', got %s", result.UnverifiedScopeSet.Slug)
		}
		if result.UnverifiedScopeSet.Description != "Unverified users can only verify their account and read their own initial data." {
			t.Errorf("Expected unverified scope set description to be 'Unverified users can only verify their account and read their own initial data.', got %s", result.UnverifiedScopeSet.Description)
		}
		if len(result.UnverifiedScopeSet.Scopes) != 2 || result.UnverifiedScopeSet.Scopes[0] != "verification:*" || result.UnverifiedScopeSet.Scopes[1] != "me:read" {
			t.Errorf("Expected unverified scope set scopes to be ['verification:*', 'me:read'], got %v", result.UnverifiedScopeSet.Scopes)
		}

		// Verify admin group was created
		if result.AdminGroup == nil {
			t.Fatal("AdminGroup should not be nil")
		}
		if result.AdminGroup.Name != useraccount.AdminGroupSlug {
			t.Errorf("Expected admin group name to be 'admin', got %s", result.AdminGroup.Name)
		}
		if result.AdminGroup.Description != "Administrator" {
			t.Errorf("Expected admin group description to be 'Administrator', got %s", result.AdminGroup.Description)
		}

		// Verify new-user group was created
		if result.NewUserGroup == nil {
			t.Fatal("NewUserGroup should not be nil")
		}
		if result.NewUserGroup.Name != useraccount.NewUserGroupSlug {
			t.Errorf("Expected new-user group name to be 'new-user', got %s", result.NewUserGroup.Name)
		}
		if result.NewUserGroup.Description != "New users that is not yet verified." {
			t.Errorf("Expected new-user group description to be 'New users that is not yet verified.', got %s", result.NewUserGroup.Description)
		}

		// Verify unverified group was created
		if result.UnverifiedGroup == nil {
			t.Fatal("UnverifiedGroup should not be nil")
		}
		if result.UnverifiedGroup.Name != useraccount.UnverifiedGroupSlug {
			t.Errorf("Expected unverified group name to be 'unverified', got %s", result.UnverifiedGroup.Name)
		}
		if result.UnverifiedGroup.Description != "Unverified users that is not yet verified." {
			t.Errorf("Expected unverified group description to be 'Unverified users that is not yet verified.', got %s", result.UnverifiedGroup.Description)
		}

		// Verify the groups are linked to the correct scope sets
		adminGroupWithScopes, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.AdminGroupSlug)).
			WithScopeSet().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin group with scope sets: %v", err)
		}
		if len(adminGroupWithScopes.Edges.ScopeSet) != 1 {
			t.Errorf("Expected admin group to have 1 scope set, got %d", len(adminGroupWithScopes.Edges.ScopeSet))
		}
		if adminGroupWithScopes.Edges.ScopeSet[0].Slug != useraccount.AdminScopeSetSlug {
			t.Errorf("Expected admin group to be linked to admin scope set, got %s", adminGroupWithScopes.Edges.ScopeSet[0].Slug)
		}

		newUserGroupWithScopes, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.NewUserGroupSlug)).
			WithScopeSet().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user group with scope sets: %v", err)
		}
		if len(newUserGroupWithScopes.Edges.ScopeSet) != 1 {
			t.Errorf("Expected new-user group to have 1 scope set, got %d", len(newUserGroupWithScopes.Edges.ScopeSet))
		}
		if newUserGroupWithScopes.Edges.ScopeSet[0].Slug != useraccount.NewUserScopeSetSlug {
			t.Errorf("Expected new-user group to be linked to new-user scope set, got %s", newUserGroupWithScopes.Edges.ScopeSet[0].Slug)
		}

		unverifiedGroupWithScopes, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).
			WithScopeSet().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query unverified group with scope sets: %v", err)
		}
		if len(unverifiedGroupWithScopes.Edges.ScopeSet) != 1 {
			t.Errorf("Expected unverified group to have 1 scope set, got %d", len(unverifiedGroupWithScopes.Edges.ScopeSet))
		}
		if unverifiedGroupWithScopes.Edges.ScopeSet[0].Slug != useraccount.UnverifiedScopeSetSlug {
			t.Errorf("Expected unverified group to be linked to unverified scope set, got %s", unverifiedGroupWithScopes.Edges.ScopeSet[0].Slug)
		}
	})

	t.Run("should be idempotent - second run should not create duplicates", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		ctx := context.Background()

		// Run setup first time
		result1, err := setup.Setup(ctx, entClient)
		if err != nil {
			t.Fatalf("First setup failed: %v", err)
		}

		// Run setup second time
		result2, err := setup.Setup(ctx, entClient)
		if err != nil {
			t.Fatalf("Second setup failed: %v", err)
		}

		// Verify that the same entities are returned (same IDs)
		if result1.AdminScopeSet.ID != result2.AdminScopeSet.ID {
			t.Errorf("Admin scope set IDs should be the same, got %d and %d", result1.AdminScopeSet.ID, result2.AdminScopeSet.ID)
		}
		if result1.NewUserScopeSet.ID != result2.NewUserScopeSet.ID {
			t.Errorf("New-user scope set IDs should be the same, got %d and %d", result1.NewUserScopeSet.ID, result2.NewUserScopeSet.ID)
		}
		if result1.UnverifiedScopeSet.ID != result2.UnverifiedScopeSet.ID {
			t.Errorf("Unverified scope set IDs should be the same, got %d and %d", result1.UnverifiedScopeSet.ID, result2.UnverifiedScopeSet.ID)
		}
		if result1.AdminGroup.ID != result2.AdminGroup.ID {
			t.Errorf("Admin group IDs should be the same, got %d and %d", result1.AdminGroup.ID, result2.AdminGroup.ID)
		}
		if result1.NewUserGroup.ID != result2.NewUserGroup.ID {
			t.Errorf("New-user group IDs should be the same, got %d and %d", result1.NewUserGroup.ID, result2.NewUserGroup.ID)
		}
		if result1.UnverifiedGroup.ID != result2.UnverifiedGroup.ID {
			t.Errorf("Unverified group IDs should be the same, got %d and %d", result1.UnverifiedGroup.ID, result2.UnverifiedGroup.ID)
		}

		// Verify that only one of each entity exists in the database
		adminScopeSets, err := entClient.ScopeSet.Query().
			Where(scopeset.SlugEQ(useraccount.AdminScopeSetSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin scope sets: %v", err)
		}
		if len(adminScopeSets) != 1 {
			t.Errorf("Expected exactly 1 admin scope set, got %d", len(adminScopeSets))
		}

		newUserScopeSets, err := entClient.ScopeSet.Query().
			Where(scopeset.SlugEQ(useraccount.NewUserScopeSetSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user scope sets: %v", err)
		}
		if len(newUserScopeSets) != 1 {
			t.Errorf("Expected exactly 1 new-user scope set, got %d", len(newUserScopeSets))
		}

		unverifiedScopeSets, err := entClient.ScopeSet.Query().
			Where(scopeset.SlugEQ(useraccount.UnverifiedScopeSetSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query unverified scope sets: %v", err)
		}
		if len(unverifiedScopeSets) != 1 {
			t.Errorf("Expected exactly 1 unverified scope set, got %d", len(unverifiedScopeSets))
		}

		adminGroups, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.AdminGroupSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin groups: %v", err)
		}
		if len(adminGroups) != 1 {
			t.Errorf("Expected exactly 1 admin group, got %d", len(adminGroups))
		}

		newUserGroups, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.NewUserGroupSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user groups: %v", err)
		}
		if len(newUserGroups) != 1 {
			t.Errorf("Expected exactly 1 new-user group, got %d", len(newUserGroups))
		}

		unverifiedGroups, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query unverified groups: %v", err)
		}
		if len(unverifiedGroups) != 1 {
			t.Errorf("Expected exactly 1 unverified group, got %d", len(unverifiedGroups))
		}
	})

	t.Run("should handle partial existing data", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		ctx := context.Background()

		// Create admin scope set manually before running setup
		existingAdminScopeSet, err := entClient.ScopeSet.Create().
			SetSlug(useraccount.AdminScopeSetSlug).
			SetDescription("Administrator").
			SetScopes([]string{"*"}).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create existing admin scope set: %v", err)
		}

		// Create unverified scope set manually before running setup
		existingUnverifiedScopeSet, err := entClient.ScopeSet.Create().
			SetSlug(useraccount.UnverifiedScopeSetSlug).
			SetDescription("Unverified users can only verify their account and read their own initial data.").
			SetScopes([]string{"verification:*", "me:read"}).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create existing unverified scope set: %v", err)
		}

		// Run setup
		result, err := setup.Setup(ctx, entClient)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify that the existing admin scope set is returned
		if result.AdminScopeSet.ID != existingAdminScopeSet.ID {
			t.Errorf("Expected to return existing admin scope set with ID %d, got %d", existingAdminScopeSet.ID, result.AdminScopeSet.ID)
		}

		// Verify that the existing unverified scope set is returned
		if result.UnverifiedScopeSet.ID != existingUnverifiedScopeSet.ID {
			t.Errorf("Expected to return existing unverified scope set with ID %d, got %d", existingUnverifiedScopeSet.ID, result.UnverifiedScopeSet.ID)
		}

		// Verify that new-user scope set was created
		if result.NewUserScopeSet == nil {
			t.Fatal("NewUserScopeSet should not be nil")
		}
		if result.NewUserScopeSet.Slug != useraccount.NewUserScopeSetSlug {
			t.Errorf("Expected new-user scope set slug to be 'new-user', got %s", result.NewUserScopeSet.Slug)
		}

		// Verify that all groups were created
		if result.AdminGroup == nil {
			t.Fatal("AdminGroup should not be nil")
		}
		if result.NewUserGroup == nil {
			t.Fatal("NewUserGroup should not be nil")
		}
		if result.UnverifiedGroup == nil {
			t.Fatal("UnverifiedGroup should not be nil")
		}
	})
}

func TestSetupResult(t *testing.T) {
	t.Run("SetupResult should have all required fields", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		ctx := context.Background()

		result, err := setup.Setup(ctx, entClient)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify all fields are populated
		if result.AdminScopeSet == nil {
			t.Error("AdminScopeSet should not be nil")
		}
		if result.NewUserScopeSet == nil {
			t.Error("NewUserScopeSet should not be nil")
		}
		if result.UnverifiedScopeSet == nil {
			t.Error("UnverifiedScopeSet should not be nil")
		}
		if result.AdminGroup == nil {
			t.Error("AdminGroup should not be nil")
		}
		if result.NewUserGroup == nil {
			t.Error("NewUserGroup should not be nil")
		}
		if result.UnverifiedGroup == nil {
			t.Error("UnverifiedGroup should not be nil")
		}
	})
}
