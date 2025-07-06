package cli_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/ent/enttest"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/scopeset"

	_ "github.com/mattn/go-sqlite3"
)

func TestSetup(t *testing.T) {
	t.Run("should create all required entities on first run", func(t *testing.T) {
		// Create an in-memory SQLite database for testing
		client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
		defer func() {
			if err := client.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()
		cliCtx := cli.NewContext(client)

		// Run setup
		result, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify admin scope set was created
		if result.AdminScopeSet == nil {
			t.Fatal("AdminScopeSet should not be nil")
		}
		if result.AdminScopeSet.Slug != "admin" {
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
		if result.NewUserScopeSet.Slug != "new-user" {
			t.Errorf("Expected new-user scope set slug to be 'new-user', got %s", result.NewUserScopeSet.Slug)
		}
		if result.NewUserScopeSet.Description != "New users can only read their own user data." {
			t.Errorf("Expected new-user scope set description to be 'New users can only read their own user data.', got %s", result.NewUserScopeSet.Description)
		}
		if len(result.NewUserScopeSet.Scopes) != 1 || result.NewUserScopeSet.Scopes[0] != "user:read" {
			t.Errorf("Expected new-user scope set scopes to be ['user:read'], got %v", result.NewUserScopeSet.Scopes)
		}

		// Verify admin group was created
		if result.AdminGroup == nil {
			t.Fatal("AdminGroup should not be nil")
		}
		if result.AdminGroup.Name != "admin" {
			t.Errorf("Expected admin group name to be 'admin', got %s", result.AdminGroup.Name)
		}
		if result.AdminGroup.Description != "Administrator" {
			t.Errorf("Expected admin group description to be 'Administrator', got %s", result.AdminGroup.Description)
		}

		// Verify new-user group was created
		if result.NewUserGroup == nil {
			t.Fatal("NewUserGroup should not be nil")
		}
		if result.NewUserGroup.Name != "new-user" {
			t.Errorf("Expected new-user group name to be 'new-user', got %s", result.NewUserGroup.Name)
		}
		if result.NewUserGroup.Description != "New users that is not yet verified." {
			t.Errorf("Expected new-user group description to be 'New users that is not yet verified.', got %s", result.NewUserGroup.Description)
		}

		// Verify the groups are linked to the correct scope sets
		adminGroupWithScopes, err := client.Group.Query().
			Where(group.NameEQ("admin")).
			WithScopeSet().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin group with scope sets: %v", err)
		}
		if len(adminGroupWithScopes.Edges.ScopeSet) != 1 {
			t.Errorf("Expected admin group to have 1 scope set, got %d", len(adminGroupWithScopes.Edges.ScopeSet))
		}
		if adminGroupWithScopes.Edges.ScopeSet[0].Slug != "admin" {
			t.Errorf("Expected admin group to be linked to admin scope set, got %s", adminGroupWithScopes.Edges.ScopeSet[0].Slug)
		}

		newUserGroupWithScopes, err := client.Group.Query().
			Where(group.NameEQ("new-user")).
			WithScopeSet().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user group with scope sets: %v", err)
		}
		if len(newUserGroupWithScopes.Edges.ScopeSet) != 1 {
			t.Errorf("Expected new-user group to have 1 scope set, got %d", len(newUserGroupWithScopes.Edges.ScopeSet))
		}
		if newUserGroupWithScopes.Edges.ScopeSet[0].Slug != "new-user" {
			t.Errorf("Expected new-user group to be linked to new-user scope set, got %s", newUserGroupWithScopes.Edges.ScopeSet[0].Slug)
		}
	})

	t.Run("should be idempotent - second run should not create duplicates", func(t *testing.T) {
		// Create an in-memory SQLite database for testing
		client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
		defer func() {
			if err := client.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()
		cliCtx := cli.NewContext(client)

		// Run setup first time
		result1, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("First setup failed: %v", err)
		}

		// Run setup second time
		result2, err := cliCtx.Setup(ctx)
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
		if result1.AdminGroup.ID != result2.AdminGroup.ID {
			t.Errorf("Admin group IDs should be the same, got %d and %d", result1.AdminGroup.ID, result2.AdminGroup.ID)
		}
		if result1.NewUserGroup.ID != result2.NewUserGroup.ID {
			t.Errorf("New-user group IDs should be the same, got %d and %d", result1.NewUserGroup.ID, result2.NewUserGroup.ID)
		}

		// Verify that only one of each entity exists in the database
		adminScopeSets, err := client.ScopeSet.Query().
			Where(scopeset.SlugEQ("admin")).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin scope sets: %v", err)
		}
		if len(adminScopeSets) != 1 {
			t.Errorf("Expected exactly 1 admin scope set, got %d", len(adminScopeSets))
		}

		newUserScopeSets, err := client.ScopeSet.Query().
			Where(scopeset.SlugEQ("new-user")).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user scope sets: %v", err)
		}
		if len(newUserScopeSets) != 1 {
			t.Errorf("Expected exactly 1 new-user scope set, got %d", len(newUserScopeSets))
		}

		adminGroups, err := client.Group.Query().
			Where(group.NameEQ("admin")).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin groups: %v", err)
		}
		if len(adminGroups) != 1 {
			t.Errorf("Expected exactly 1 admin group, got %d", len(adminGroups))
		}

		newUserGroups, err := client.Group.Query().
			Where(group.NameEQ("new-user")).
			All(ctx)
		if err != nil {
			t.Fatalf("Failed to query new-user groups: %v", err)
		}
		if len(newUserGroups) != 1 {
			t.Errorf("Expected exactly 1 new-user group, got %d", len(newUserGroups))
		}
	})

	t.Run("should handle partial existing data", func(t *testing.T) {
		// Create an in-memory SQLite database for testing
		client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
		defer func() {
			if err := client.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()
		cliCtx := cli.NewContext(client)

		// Create admin scope set manually before running setup
		existingAdminScopeSet, err := client.ScopeSet.Create().
			SetSlug("admin").
			SetDescription("Administrator").
			SetScopes([]string{"*"}).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create existing admin scope set: %v", err)
		}

		// Run setup
		result, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify that the existing admin scope set is returned
		if result.AdminScopeSet.ID != existingAdminScopeSet.ID {
			t.Errorf("Expected to return existing admin scope set with ID %d, got %d", existingAdminScopeSet.ID, result.AdminScopeSet.ID)
		}

		// Verify that new-user scope set was created
		if result.NewUserScopeSet == nil {
			t.Fatal("NewUserScopeSet should not be nil")
		}
		if result.NewUserScopeSet.Slug != "new-user" {
			t.Errorf("Expected new-user scope set slug to be 'new-user', got %s", result.NewUserScopeSet.Slug)
		}

		// Verify that both groups were created
		if result.AdminGroup == nil {
			t.Fatal("AdminGroup should not be nil")
		}
		if result.NewUserGroup == nil {
			t.Fatal("NewUserGroup should not be nil")
		}
	})
}

func TestSetupResult(t *testing.T) {
	t.Run("SetupResult should have all required fields", func(t *testing.T) {
		// Create an in-memory SQLite database for testing
		client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
		defer func() {
			if err := client.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()
		cliCtx := cli.NewContext(client)

		result, err := cliCtx.Setup(ctx)
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
		if result.AdminGroup == nil {
			t.Error("AdminGroup should not be nil")
		}
		if result.NewUserGroup == nil {
			t.Error("NewUserGroup should not be nil")
		}
	})
}
