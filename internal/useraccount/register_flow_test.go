package useraccount_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrRegister_NewUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	req := useraccount.UserRegisterRequest{
		Name:  "Test User",
		Email: "test1@example.com", // Unique email
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Verify user was created with correct data
	assert.Equal(t, req.Name, user.Name)
	assert.Equal(t, req.Email, user.Email)

	// Verify user is in unverified group
	group, err := user.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.UnverifiedGroupSlug, group.Name)

	// Verify user has unverified scope
	scopeSets, err := user.QueryGroup().QueryScopeSets().All(context)
	require.NoError(t, err)
	require.Len(t, scopeSets, 1)
	assert.Contains(t, scopeSets[0].Scopes, "unverified")
}

func TestGetOrRegister_ExistingUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create an existing user
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	existingUser, err := client.User.Create().
		SetName("Existing User").
		SetEmail("existing2@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	req := useraccount.UserRegisterRequest{
		Name:  "Different Name",        // Different name
		Email: "existing2@example.com", // Same email
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Should return existing user, not create new one
	assert.Equal(t, existingUser.ID, user.ID)
	assert.Equal(t, req.Name, user.Name) // Should update to new name from OAuth
	assert.Equal(t, existingUser.Email, user.Email)
}

func TestGetOrRegister_UpdateNameAndAvatar(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create an existing user with original name and avatar
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	originalName := "Original Name"
	originalAvatar := "https://example.com/old-avatar.jpg"
	existingUser, err := client.User.Create().
		SetName(originalName).
		SetEmail("update-test@example.com").
		SetAvatar(originalAvatar).
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Register again with updated OAuth info
	updatedName := "Updated OAuth Name"
	updatedAvatar := "https://oauth-provider.com/new-avatar.jpg"
	req := useraccount.UserRegisterRequest{
		Name:   updatedName,
		Email:  "update-test@example.com", // Same email
		Avatar: updatedAvatar,
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Verify it's the same user (ID unchanged)
	assert.Equal(t, existingUser.ID, user.ID)

	// Verify name and avatar were updated to match OAuth info
	assert.Equal(t, updatedName, user.Name, "name should be updated to match OAuth info")
	assert.Equal(t, updatedAvatar, user.Avatar, "avatar should be updated to match OAuth info")
	assert.Equal(t, existingUser.Email, user.Email, "email should remain unchanged")

	// Verify the updates persisted in the database
	refreshedUser, err := client.User.Get(context, user.ID)
	require.NoError(t, err)
	assert.Equal(t, updatedName, refreshedUser.Name)
	assert.Equal(t, updatedAvatar, refreshedUser.Avatar)
}

func TestGetOrRegister_UpdateNameAndAvatar_VerifiedUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create an existing verified user (in student group)
	studentGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.StudentGroupSlug)).Only(context)
	require.NoError(t, err)

	originalName := "Verified User Original"
	originalAvatar := "https://example.com/verified-old.jpg"
	existingUser, err := client.User.Create().
		SetName(originalName).
		SetEmail("verified-update@example.com").
		SetAvatar(originalAvatar).
		SetGroup(studentGroup).
		Save(context)
	require.NoError(t, err)

	// Login/register again with updated OAuth info
	updatedName := "Verified User Updated"
	updatedAvatar := "https://oauth-provider.com/verified-new.jpg"
	req := useraccount.UserRegisterRequest{
		Name:   updatedName,
		Email:  "verified-update@example.com",
		Avatar: updatedAvatar,
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Verify the existing verified user was updated
	assert.Equal(t, existingUser.ID, user.ID)
	assert.Equal(t, updatedName, user.Name, "name should be updated even for verified users")
	assert.Equal(t, updatedAvatar, user.Avatar, "avatar should be updated even for verified users")

	// Verify user is still in the verified group
	group, err := user.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.StudentGroupSlug, group.Name, "group should not change")
}

func TestGetOrRegister_UpdateWithEmptyAvatar(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create user with avatar
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	existingUser, err := client.User.Create().
		SetName("User With Avatar").
		SetEmail("avatar-test@example.com").
		SetAvatar("https://example.com/has-avatar.jpg").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Register again with empty avatar (OAuth provider might not provide avatar)
	req := useraccount.UserRegisterRequest{
		Name:   "User Name Updated",
		Email:  "avatar-test@example.com",
		Avatar: "", // Empty avatar
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Verify avatar was cleared
	assert.Equal(t, existingUser.ID, user.ID)
	assert.Equal(t, req.Name, user.Name)
	assert.Equal(t, "", user.Avatar, "empty avatar should be set")
}

func TestGetOrRegister_MissingUnverifiedGroup(t *testing.T) {
	// Create a fresh database without setup
	client := testhelper.NewEntSqliteClient(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	req := useraccount.UserRegisterRequest{
		Name:  "Test User",
		Email: "test3@example.com", // Unique email
	}

	_, err := ctx.GetOrRegister(context, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, useraccount.ErrIncompleteSetup)
}

func TestVerify_Success(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create an unverified user
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test4@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Verify the user
	err = ctx.Verify(context, user.ID)
	require.NoError(t, err)

	// Check that user is now in student group
	updatedUser, err := client.User.Get(context, user.ID)
	require.NoError(t, err)

	group, err := updatedUser.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.StudentGroupSlug, group.Name)

	// Verify user has student scopes
	scopeSets, err := updatedUser.QueryGroup().QueryScopeSets().All(context)
	require.NoError(t, err)
	require.Len(t, scopeSets, 1)
	assert.Contains(t, scopeSets[0].Scopes, "me:*")
}

func TestVerify_UserNotFound(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	err := ctx.Verify(context, 99999) // Non-existent user ID
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get user")
}

func TestVerify_UserAlreadyVerified(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in student group (already verified)
	studentGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.StudentGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Verified User").
		SetEmail("verified5@example.com"). // Unique email
		SetGroup(studentGroup).
		Save(context)
	require.NoError(t, err)

	// Try to verify again
	err = ctx.Verify(context, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, useraccount.ErrUserVerified)
}

func TestVerify_MissingStudentGroup(t *testing.T) {
	// Create a fresh database without setup
	client := testhelper.NewEntSqliteClient(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create only unverified group
	unverifiedGroup, err := client.Group.Create().
		SetName(useraccount.UnverifiedGroupSlug).
		SetDescription("Unverified users").
		Save(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test6@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Try to verify - should fail due to missing student group
	err = ctx.Verify(context, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, useraccount.ErrIncompleteSetup)
}

func TestRegistrationFlow_Complete(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Step 1: Register new user (should be unverified)
	req := useraccount.UserRegisterRequest{
		Name:  "John Doe",
		Email: "john10@example.com", // Unique email
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)

	// Verify user is in unverified group
	group, err := user.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.UnverifiedGroupSlug, group.Name)

	// Step 2: Grant token for unverified user
	token, err := ctx.GrantToken(context, user, "web", useraccount.WithFlow("registration"))
	require.NoError(t, err)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "unverified")

	// Step 3: Verify the user
	err = ctx.Verify(context, user.ID)
	require.NoError(t, err)

	// Step 4: Verify user is now in student group
	updatedUser, err := client.User.Get(context, user.ID)
	require.NoError(t, err)

	updatedGroup, err := updatedUser.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.StudentGroupSlug, updatedGroup.Name)

	// Step 5: Grant token for verified user
	newToken, err := ctx.GrantToken(context, updatedUser, "web", useraccount.WithFlow("login"))
	require.NoError(t, err)

	newTokenInfo, err := authStorage.Get(context, newToken)
	require.NoError(t, err)
	assert.Contains(t, newTokenInfo.Scopes, "me:*")
}

func TestRegistrationFlow_ExistingUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create an existing verified user
	studentGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.StudentGroupSlug)).Only(context)
	require.NoError(t, err)

	existingUser, err := client.User.Create().
		SetName("Existing User").
		SetEmail("existing11@example.com"). // Unique email
		SetGroup(studentGroup).
		Save(context)
	require.NoError(t, err)

	// Try to register same user again
	req := useraccount.UserRegisterRequest{
		Name:  "Different Name",
		Email: "existing11@example.com", // Same email
	}

	user, err := ctx.GetOrRegister(context, req)
	require.NoError(t, err)

	// Should return existing user
	assert.Equal(t, existingUser.ID, user.ID)
	assert.Equal(t, req.Name, user.Name) // Name updated to match OAuth info

	// Grant token - should have student scopes
	token, err := ctx.GrantToken(context, user, "web", useraccount.WithFlow("login"))
	require.NoError(t, err)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "me:*")
}

func TestRegistrationFlow_ErrorCases(t *testing.T) {
	t.Run("incomplete setup - missing groups", func(t *testing.T) {
		// Create a fresh database without setup
		client := testhelper.NewEntSqliteClient(t)
		authStorage := newMockAuthStorage()
		eventService := events.NewEventService(client, nil)
		ctx := useraccount.NewContext(client, authStorage, eventService)
		context := context.Background()

		req := useraccount.UserRegisterRequest{
			Name:  "Test User",
			Email: "test12@example.com", // Unique email
		}

		_, err := ctx.GetOrRegister(context, req)
		require.Error(t, err)
		assert.ErrorIs(t, err, useraccount.ErrIncompleteSetup)
	})

	t.Run("verify already verified user", func(t *testing.T) {
		client := setupTestDatabase(t)
		authStorage := newMockAuthStorage()
		eventService := events.NewEventService(client, nil)
		ctx := useraccount.NewContext(client, authStorage, eventService)
		context := context.Background()

		// Create verified user
		studentGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.StudentGroupSlug)).Only(context)
		require.NoError(t, err)

		user, err := client.User.Create().
			SetName("Verified User").
			SetEmail("verified13@example.com"). // Unique email
			SetGroup(studentGroup).
			Save(context)
		require.NoError(t, err)

		// Try to verify again
		err = ctx.Verify(context, user.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, useraccount.ErrUserVerified)
	})

	t.Run("verify non-existent user", func(t *testing.T) {
		client := setupTestDatabase(t)
		authStorage := newMockAuthStorage()
		eventService := events.NewEventService(client, nil)
		ctx := useraccount.NewContext(client, authStorage, eventService)
		context := context.Background()

		err := ctx.Verify(context, 99999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get user")
	})
}

func TestUserRegisterRequest_Validation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := useraccount.UserRegisterRequest{
			Name:  "Test User",
			Email: "test@example.com",
		}
		// Should not panic
		_ = req
	})

	t.Run("empty name", func(t *testing.T) {
		req := useraccount.UserRegisterRequest{
			Name:  "",
			Email: "test@example.com",
		}
		// Should not panic
		_ = req
	})

	t.Run("empty email", func(t *testing.T) {
		req := useraccount.UserRegisterRequest{
			Name:  "Test User",
			Email: "",
		}
		// Should not panic
		_ = req
	})
}
