package cli_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"

	_ "github.com/mattn/go-sqlite3"
)

func TestPromoteAdmin(t *testing.T) {
	t.Run("should successfully promote user to admin", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Get the student group to assign to the user initially
		studentGroup, err := entClient.Group.Query().Where(group.Name(useraccount.StudentGroupSlug)).Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		// Create a test user
		testUser, err := entClient.User.Create().
			SetEmail("test@example.com").
			SetName("Test User").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Verify user is not in admin group initially
		userWithGroup, err := entClient.User.Query().
			Where(user.ID(testUser.ID)).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user with group: %v", err)
		}
		if userWithGroup.Edges.Group == nil {
			t.Fatal("User should be in a group initially")
		}
		if userWithGroup.Edges.Group.Name == "admin" {
			t.Errorf("User should not be in admin group initially, but was in group: %s", userWithGroup.Edges.Group.Name)
		}

		// Promote the user to admin
		err = cliCtx.PromoteAdmin(ctx, "test@example.com")
		if err != nil {
			t.Fatalf("PromoteAdmin failed: %v", err)
		}

		// Verify user is now in admin group
		userWithGroup, err = entClient.User.Query().
			Where(user.ID(testUser.ID)).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user with group after promotion: %v", err)
		}
		if userWithGroup.Edges.Group == nil {
			t.Fatal("User should be in admin group after promotion")
		}
		if userWithGroup.Edges.Group.Name != "admin" {
			t.Errorf("Expected user to be in admin group, got: %s", userWithGroup.Edges.Group.Name)
		}
	})

	t.Run("should return error when user not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Try to promote non-existent user
		err = cliCtx.PromoteAdmin(ctx, "nonexistent@example.com")
		if err == nil {
			t.Fatal("Expected error when promoting non-existent user")
		}

		expectedErrMsg := "user with email \"nonexistent@example.com\" not found"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error when admin group not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Create a test user without running setup (so no admin group exists)
		// We need to create a group first since User requires a group
		testGroup, err := entClient.Group.Create().
			SetName("test-group").
			SetDescription("Test group").
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create test group: %v", err)
		}

		_, err = entClient.User.Create().
			SetEmail("test@example.com").
			SetName("Test User").
			SetGroup(testGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Try to promote user when admin group doesn't exist
		err = cliCtx.PromoteAdmin(ctx, "test@example.com")
		if err == nil {
			t.Fatal("Expected error when admin group doesn't exist")
		}

		expectedErrMsg := "admin group not found; run \"setup\" first"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should handle database errors gracefully", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Get the student group to assign to the user initially
		studentGroup, err := entClient.Group.Query().Where(group.Name(useraccount.StudentGroupSlug)).Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		// Create a test user
		_, err = entClient.User.Create().
			SetEmail("test@example.com").
			SetName("Test User").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Close the client to simulate database connection issues
		if err := entClient.Close(); err != nil {
			t.Fatalf("Failed to close client: %v", err)
		}

		// Try to promote user with closed client
		err = cliCtx.PromoteAdmin(ctx, "test@example.com")
		if err == nil {
			t.Fatal("Expected error when database is closed")
		}
	})

	t.Run("should work with multiple users", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Get the student group to assign to users initially
		studentGroup, err := entClient.Group.Query().Where(group.Name(useraccount.StudentGroupSlug)).Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		// Create multiple test users
		user1, err := entClient.User.Create().
			SetEmail("user1@example.com").
			SetName("User 1").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create user1: %v", err)
		}

		user2, err := entClient.User.Create().
			SetEmail("user2@example.com").
			SetName("User 2").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create user2: %v", err)
		}

		// Promote first user
		err = cliCtx.PromoteAdmin(ctx, "user1@example.com")
		if err != nil {
			t.Fatalf("Failed to promote user1: %v", err)
		}

		// Promote second user
		err = cliCtx.PromoteAdmin(ctx, "user2@example.com")
		if err != nil {
			t.Fatalf("Failed to promote user2: %v", err)
		}

		// Verify both users are in admin group
		user1WithGroup, err := entClient.User.Query().
			Where(user.ID(user1.ID)).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user1 with group: %v", err)
		}
		if user1WithGroup.Edges.Group == nil || user1WithGroup.Edges.Group.Name != "admin" {
			t.Errorf("User1 should be in admin group, got: %v", user1WithGroup.Edges.Group)
		}

		user2WithGroup, err := entClient.User.Query().
			Where(user.ID(user2.ID)).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user2 with group: %v", err)
		}
		if user2WithGroup.Edges.Group == nil || user2WithGroup.Edges.Group.Name != "admin" {
			t.Errorf("User2 should be in admin group, got: %v", user2WithGroup.Edges.Group)
		}

		// Verify admin group exists and both users are in it
		adminGroup, err := entClient.Group.Query().
			Where(group.Name("admin")).
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query admin group: %v", err)
		}
		if adminGroup == nil {
			t.Fatal("Admin group should exist")
		}
	})

	t.Run("should handle case sensitivity in email", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Get the student group to assign to the user initially
		studentGroup, err := entClient.Group.Query().Where(group.Name(useraccount.StudentGroupSlug)).Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		// Create a test user with lowercase email
		_, err = entClient.User.Create().
			SetEmail("test@example.com").
			SetName("Test User").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Try to promote with different case (should fail if case sensitive)
		err = cliCtx.PromoteAdmin(ctx, "TEST@EXAMPLE.COM")
		if err == nil {
			t.Fatal("Expected error when using different case for email")
		}

		expectedErrMsg := "user with email \"TEST@EXAMPLE.COM\" not found"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}

		// Try with correct case
		err = cliCtx.PromoteAdmin(ctx, "test@example.com")
		if err != nil {
			t.Fatalf("Failed to promote user with correct case: %v", err)
		}
	})

	t.Run("should handle empty email", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// Run setup to create admin group
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Try to promote with empty email
		err = cliCtx.PromoteAdmin(ctx, "")
		if err == nil {
			t.Fatal("Expected error when using empty email")
		}

		expectedErrMsg := "email is required"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})
}
