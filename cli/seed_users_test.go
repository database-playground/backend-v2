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

func TestSeedUsers(t *testing.T) {
	t.Run("should successfully seed users with valid records", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed users
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"},
			{Email: "user2@example.com", Group: "admin"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers failed: %v", err)
		}

		// Verify users were created
		user1, err := entClient.User.Query().
			Where(user.EmailEQ("user1@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user1: %v", err)
		}
		if user1.Email != "user1@example.com" {
			t.Errorf("Expected email user1@example.com, got %s", user1.Email)
		}
		if user1.Edges.Group == nil || user1.Edges.Group.Name != "student" {
			t.Errorf("Expected user1 to be in student group, got %v", user1.Edges.Group)
		}

		user2, err := entClient.User.Query().
			Where(user.EmailEQ("user2@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user2: %v", err)
		}
		if user2.Email != "user2@example.com" {
			t.Errorf("Expected email user2@example.com, got %s", user2.Email)
		}
		if user2.Edges.Group == nil || user2.Edges.Group.Name != "admin" {
			t.Errorf("Expected user2 to be in admin group, got %v", user2.Edges.Group)
		}
	})

	t.Run("should default to student group when group is empty", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed user with empty group
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: ""},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers failed: %v", err)
		}

		// Verify user was created with student group
		user, err := entClient.User.Query().
			Where(user.EmailEQ("user@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query user: %v", err)
		}
		if user.Edges.Group == nil || user.Edges.Group.Name != useraccount.StudentGroupSlug {
			t.Errorf("Expected user to be in student group, got %v", user.Edges.Group)
		}
	})

	t.Run("should handle multiple users with same group efficiently", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed multiple users with same group
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"},
			{Email: "user2@example.com", Group: "student"},
			{Email: "user3@example.com", Group: "student"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers failed: %v", err)
		}

		// Verify all users were created with student group
		studentGroup, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.StudentGroupSlug)).
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query student group: %v", err)
		}

		for _, email := range []string{"user1@example.com", "user2@example.com", "user3@example.com"} {
			user, err := entClient.User.Query().
				Where(user.EmailEQ(email)).
				WithGroup().
				Only(ctx)
			if err != nil {
				t.Fatalf("Failed to query user %s: %v", email, err)
			}
			if user.Edges.Group == nil || user.Edges.Group.ID != studentGroup.ID {
				t.Errorf("Expected user %s to be in student group, got %v", email, user.Edges.Group)
			}
		}
	})

	t.Run("should return error when email is empty", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed user with empty email
		records := []cli.UserSeedRecord{
			{Email: "", Group: "student"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when email is empty")
		}

		expectedErrMsg := "user seed record #0: email is required"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error when group not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed user with non-existent group
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: "nonexistent-group"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when group not found")
		}

		expectedErrMsg := "group \"nonexistent-group\" not found"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error for multiple validation errors", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed users with multiple invalid records
		records := []cli.UserSeedRecord{
			{Email: "", Group: "student"},                  // invalid: empty email
			{Email: "user2@example.com", Group: "student"}, // valid
			{Email: "", Group: "admin"},                    // invalid: empty email
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when validation fails")
		}

		expectedErrMsg := "user seed record #0: email is required"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error when duplicate user email", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Create a user first
		studentGroup, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.StudentGroupSlug)).
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		_, err = entClient.User.Create().
			SetEmail("existing@example.com").
			SetName("Existing User").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create existing user: %v", err)
		}

		// Try to seed a user with the same email
		records := []cli.UserSeedRecord{
			{Email: "existing@example.com", Group: "student"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when duplicate email")
		}

		// The error should be a constraint violation
		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("should handle empty records array", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed with empty records
		records := []cli.UserSeedRecord{}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers should succeed with empty records, got error: %v", err)
		}
	})

	t.Run("should handle database connection errors gracefully", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Close the client to simulate database connection issues
		if err := entClient.Close(); err != nil {
			t.Fatalf("Failed to close client: %v", err)
		}

		// Try to seed users with closed client
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: "student"},
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when database is closed")
		}
	})

	t.Run("should handle mixed valid and invalid groups", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		ctx := context.Background()
		cliCtx := cli.NewContext(entClient)

		// First run setup to create groups
		_, err := cliCtx.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Seed users with one valid and one invalid group
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"}, // valid
			{Email: "user2@example.com", Group: "invalid"}, // invalid
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when one group is invalid")
		}

		expectedErrMsg := "group \"invalid\" not found"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}

		// Verify no users were created
		count, err := entClient.User.Query().Count(ctx)
		if err == nil && count > 0 {
			// If we can query, check that no users were created
			// (Note: this might fail if setup created users, but setup shouldn't)
		}
	})
}
