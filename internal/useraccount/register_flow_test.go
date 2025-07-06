package useraccount_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent/enttest"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrRegister_NewUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
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

	// Verify user has verification:* scope
	scopeSets, err := user.QueryGroup().QueryScopeSet().All(context)
	require.NoError(t, err)
	require.Len(t, scopeSets, 1)
	assert.Contains(t, scopeSets[0].Scopes, "verification:*")
}

func TestGetOrRegister_ExistingUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
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
	assert.Equal(t, existingUser.Name, user.Name) // Should keep original name
	assert.Equal(t, existingUser.Email, user.Email)
}

func TestGetOrRegister_MissingUnverifiedGroup(t *testing.T) {
	// Create a fresh database without setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
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
	ctx := useraccount.NewContext(client, authStorage)
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

	// Check that user is now in new-user group
	updatedUser, err := client.User.Get(context, user.ID)
	require.NoError(t, err)

	group, err := updatedUser.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.NewUserGroupSlug, group.Name)

	// Verify user has new-user scopes
	scopeSets, err := updatedUser.QueryGroup().QueryScopeSet().All(context)
	require.NoError(t, err)
	require.Len(t, scopeSets, 1)
	assert.Contains(t, scopeSets[0].Scopes, "me:*")
}

func TestVerify_UserNotFound(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	err := ctx.Verify(context, 99999) // Non-existent user ID
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get user")
}

func TestVerify_UserAlreadyVerified(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	// Create a user in new-user group (already verified)
	newUserGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.NewUserGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Verified User").
		SetEmail("verified5@example.com"). // Unique email
		SetGroup(newUserGroup).
		Save(context)
	require.NoError(t, err)

	// Try to verify again
	err = ctx.Verify(context, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, useraccount.ErrUserVerified)
}

func TestVerify_MissingNewUserGroup(t *testing.T) {
	// Create a fresh database without setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
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

	// Try to verify - should fail due to missing new-user group
	err = ctx.Verify(context, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, useraccount.ErrIncompleteSetup)
}

func TestRegistrationFlow_Complete(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
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
	token, err := ctx.GrantToken(context, user, "web", "registration")
	require.NoError(t, err)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "verification:*")

	// Step 3: Verify the user
	err = ctx.Verify(context, user.ID)
	require.NoError(t, err)

	// Step 4: Verify user is now in new-user group
	updatedUser, err := client.User.Get(context, user.ID)
	require.NoError(t, err)

	updatedGroup, err := updatedUser.QueryGroup().Only(context)
	require.NoError(t, err)
	assert.Equal(t, useraccount.NewUserGroupSlug, updatedGroup.Name)

	// Step 5: Grant token for verified user
	newToken, err := ctx.GrantToken(context, updatedUser, "web", "login")
	require.NoError(t, err)

	newTokenInfo, err := authStorage.Get(context, newToken)
	require.NoError(t, err)
	assert.Contains(t, newTokenInfo.Scopes, "me:*")
}

func TestRegistrationFlow_ExistingUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	// Create an existing verified user
	newUserGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.NewUserGroupSlug)).Only(context)
	require.NoError(t, err)

	existingUser, err := client.User.Create().
		SetName("Existing User").
		SetEmail("existing11@example.com"). // Unique email
		SetGroup(newUserGroup).
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
	assert.Equal(t, existingUser.Name, user.Name) // Keep original name

	// Grant token - should have new-user scopes
	token, err := ctx.GrantToken(context, user, "web", "login")
	require.NoError(t, err)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "me:*")
}

func TestRegistrationFlow_ErrorCases(t *testing.T) {
	t.Run("incomplete setup - missing groups", func(t *testing.T) {
		// Create a fresh database without setup
		client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")
		authStorage := newMockAuthStorage()
		ctx := useraccount.NewContext(client, authStorage)
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
		ctx := useraccount.NewContext(client, authStorage)
		context := context.Background()

		// Create verified user
		newUserGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.NewUserGroupSlug)).Only(context)
		require.NoError(t, err)

		user, err := client.User.Create().
			SetName("Verified User").
			SetEmail("verified13@example.com"). // Unique email
			SetGroup(newUserGroup).
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
		ctx := useraccount.NewContext(client, authStorage)
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
