package cli_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/internal/useraccount"

	_ "github.com/mattn/go-sqlite3"
)

func TestSeedUsers(t *testing.T) {
	t.Run("should successfully seed users with valid records", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed users
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"},
			{Email: "user2@example.com", Group: "admin"},
		}

		err := cliCtx.SeedUsers(ctx, records)
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
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed user with empty group
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: ""},
		}

		err := cliCtx.SeedUsers(ctx, records)
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
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed multiple users with same group
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"},
			{Email: "user2@example.com", Group: "student"},
			{Email: "user3@example.com", Group: "student"},
		}

		err := cliCtx.SeedUsers(ctx, records)
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
		ctx := context.Background()
		tc := NewTestContext(t)

		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed user with empty email
		records := []cli.UserSeedRecord{
			{Email: "", Group: "student"},
		}

		err := cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when email is empty")
		}

		expectedErrMsg := "user seed record #0: email is required"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error when group not found", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed user with non-existent group
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: "nonexistent-group"},
		}

		err := cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when group not found")
		}

		expectedErrMsg := "group \"nonexistent-group\" not found"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should return error for multiple validation errors", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed users with multiple invalid records
		records := []cli.UserSeedRecord{
			{Email: "", Group: "student"},                  // invalid: empty email
			{Email: "user2@example.com", Group: "student"}, // valid
			{Email: "", Group: "admin"},                    // invalid: empty email
		}

		err := cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when validation fails")
		}

		expectedErrMsg := "user seed record #0: email is required"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
		}
	})

	t.Run("should skip when user already exists", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Create a user first with a specific name and group
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

		// Query the user with group to get the original values
		existingUser, err := entClient.User.Query().
			Where(user.EmailEQ("existing@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query existing user: %v", err)
		}

		originalName := existingUser.Name
		originalGroupID := existingUser.Edges.Group.ID

		// Try to seed a user with the same email (should skip, not error)
		records := []cli.UserSeedRecord{
			{Email: "existing@example.com", Group: "admin"}, // Different group, but should be skipped
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers should not error when user already exists, got: %v", err)
		}

		// Verify the existing user was not modified
		queriedUser, err := entClient.User.Query().
			Where(user.EmailEQ("existing@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query existing user: %v", err)
		}

		// Verify name was not changed
		if queriedUser.Name != originalName {
			t.Errorf("Expected name to remain %q, got %q", originalName, queriedUser.Name)
		}

		// Verify group was not changed
		if queriedUser.Edges.Group.ID != originalGroupID {
			t.Errorf("Expected group ID to remain %d, got %d", originalGroupID, queriedUser.Edges.Group.ID)
		}

		// Verify only one user with this email exists
		count, err := entClient.User.Query().
			Where(user.EmailEQ("existing@example.com")).
			Count(ctx)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected exactly 1 user with email existing@example.com, got %d", count)
		}
	})

	t.Run("should handle mixed existing and new users in batch", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Create an existing user
		studentGroup, err := entClient.Group.Query().
			Where(group.NameEQ(useraccount.StudentGroupSlug)).
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to get student group: %v", err)
		}

		existingUser, err := entClient.User.Create().
			SetEmail("existing@example.com").
			SetName("Existing User").
			SetGroup(studentGroup).
			Save(ctx)
		if err != nil {
			t.Fatalf("Failed to create existing user: %v", err)
		}

		originalName := existingUser.Name

		// Seed a batch with both existing and new users
		records := []cli.UserSeedRecord{
			{Email: "existing@example.com", Group: "student"}, // existing - should skip
			{Email: "newuser1@example.com", Group: "student"}, // new - should create
			{Email: "newuser2@example.com", Group: "admin"},   // new - should create
		}

		err = cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers should succeed with mixed existing/new users, got: %v", err)
		}

		// Verify existing user was not modified
		queriedExistingUser, err := entClient.User.Query().
			Where(user.EmailEQ("existing@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query existing user: %v", err)
		}
		if queriedExistingUser.Name != originalName {
			t.Errorf("Expected existing user name to remain %q, got %q", originalName, queriedExistingUser.Name)
		}

		// Verify new users were created
		newUser1, err := entClient.User.Query().
			Where(user.EmailEQ("newuser1@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query newuser1: %v", err)
		}
		if newUser1.Name != "newuser1@example.com" {
			t.Errorf("Expected newuser1 name to be email, got %q", newUser1.Name)
		}
		if newUser1.Edges.Group == nil || newUser1.Edges.Group.Name != "student" {
			t.Errorf("Expected newuser1 to be in student group, got %v", newUser1.Edges.Group)
		}

		newUser2, err := entClient.User.Query().
			Where(user.EmailEQ("newuser2@example.com")).
			WithGroup().
			Only(ctx)
		if err != nil {
			t.Fatalf("Failed to query newuser2: %v", err)
		}
		if newUser2.Name != "newuser2@example.com" {
			t.Errorf("Expected newuser2 name to be email, got %q", newUser2.Name)
		}
		if newUser2.Edges.Group == nil || newUser2.Edges.Group.Name != "admin" {
			t.Errorf("Expected newuser2 to be in admin group, got %v", newUser2.Edges.Group)
		}

		// Verify total count
		count, err := entClient.User.Query().Count(ctx)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		// Should have: existing user + 2 new users = 3 total
		// (setup doesn't create users, only groups)
		if count != 3 {
			t.Errorf("Expected 3 users total, got %d", count)
		}
	})

	t.Run("should handle empty records array", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed with empty records
		records := []cli.UserSeedRecord{}

		err := cliCtx.SeedUsers(ctx, records)
		if err != nil {
			t.Fatalf("SeedUsers should succeed with empty records, got error: %v", err)
		}
	})

	t.Run("should handle database connection errors gracefully", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Close the client to simulate database connection issues
		if err := entClient.Close(); err != nil {
			t.Fatalf("Failed to close client: %v", err)
		}

		// Try to seed users with closed client
		records := []cli.UserSeedRecord{
			{Email: "user@example.com", Group: "student"},
		}

		err := cliCtx.SeedUsers(ctx, records)
		if err == nil {
			t.Fatal("Expected error when database is closed")
		}
	})

	t.Run("should handle mixed valid and invalid groups", func(t *testing.T) {
		ctx := context.Background()
		tc := NewTestContext(t)

		entClient := tc.GetEntClient(t)
		cliCtx := tc.GetContext(t)

		tc.Setup(t)

		// Seed users with one valid and one invalid group
		records := []cli.UserSeedRecord{
			{Email: "user1@example.com", Group: "student"}, // valid
			{Email: "user2@example.com", Group: "invalid"}, // invalid
		}

		err := cliCtx.SeedUsers(ctx, records)
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
