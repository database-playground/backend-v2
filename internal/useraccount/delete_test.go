package useraccount_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/require"
)

func TestDeleteUser(t *testing.T) {
	tests := []struct {
		name        string
		setupUser   bool
		userID      int
		expectError error
	}{
		{
			name:        "successfully delete existing user",
			setupUser:   true,
			userID:      1,
			expectError: nil,
		},
		{
			name:        "fail to delete non-existent user",
			setupUser:   false,
			userID:      999,
			expectError: useraccount.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupTestDatabase(t)
			authStorage := newMockAuthStorage()
			ctx := useraccount.NewContext(client, authStorage)

			var userID int
			if tt.setupUser {
				// Get the unverified group
				unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context.Background())
				require.NoError(t, err)

				// Create a test user
				user, err := client.User.Create().
					SetEmail("test@example.com").
					SetName("Test User").
					SetGroup(unverifiedGroup).
					Save(context.Background())
				require.NoError(t, err)
				userID = user.ID
			} else {
				userID = tt.userID
			}

			// Attempt to delete the user
			err := ctx.DeleteUser(context.Background(), userID)

			if tt.expectError != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectError)
			} else {
				require.NoError(t, err)

				// Verify the user was actually deleted
				_, err := client.User.Get(context.Background(), userID)
				require.Error(t, err)
				require.True(t, ent.IsNotFound(err))
			}
		})
	}
}

func TestDeleteUser_Integration(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)

	// Get the unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context.Background())
	require.NoError(t, err)

	// Create multiple users
	user1, err := client.User.Create().
		SetEmail("user1@example.com").
		SetName("User 1").
		SetGroup(unverifiedGroup).
		Save(context.Background())
	require.NoError(t, err)

	user2, err := client.User.Create().
		SetEmail("user2@example.com").
		SetName("User 2").
		SetGroup(unverifiedGroup).
		Save(context.Background())
	require.NoError(t, err)

	// Verify both users exist
	_, err = client.User.Get(context.Background(), user1.ID)
	require.NoError(t, err)
	_, err = client.User.Get(context.Background(), user2.ID)
	require.NoError(t, err)

	// Delete first user
	err = ctx.DeleteUser(context.Background(), user1.ID)
	require.NoError(t, err)

	// Verify first user is deleted
	_, err = client.User.Get(context.Background(), user1.ID)
	require.Error(t, err)
	require.True(t, ent.IsNotFound(err))

	// Verify second user still exists
	_, err = client.User.Get(context.Background(), user2.ID)
	require.NoError(t, err)

	// Try to delete the same user again
	err = ctx.DeleteUser(context.Background(), user1.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, useraccount.ErrUserNotFound)
}
