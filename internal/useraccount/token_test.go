package useraccount_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantToken_Success(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test7@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token
	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("registration"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify token was created with correct info
	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, tokenInfo.UserID)
	assert.Equal(t, user.Email, tokenInfo.UserEmail)
	assert.Equal(t, "test-machine", tokenInfo.Machine)
	assert.Contains(t, tokenInfo.Scopes, "verification:*")
	assert.Equal(t, "registration", tokenInfo.Meta["initiate_from_flow"])
}

func TestGrantToken_Impersonation(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test8@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("registration"),
		useraccount.WithImpersonation(user.ID),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, tokenInfo.UserID)
	assert.Equal(t, user.Email, tokenInfo.UserEmail)
	assert.Equal(t, "test-machine", tokenInfo.Machine)
	assert.Contains(t, tokenInfo.Scopes, "verification:*")
	assert.Equal(t, "registration", tokenInfo.Meta["initiate_from_flow"])
	assert.Equal(t, strconv.Itoa(user.ID), tokenInfo.Meta["impersonation"])
}

func TestGrantToken_DefaultFlow(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test9@example.com"). // Unique email
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	token, err := ctx.GrantToken(
		context, user, "test-machine",
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Equal(t, "undefined", tokenInfo.Meta["initiate_from_flow"])
	assert.Empty(t, tokenInfo.Meta["impersonation"])
}

func TestGrantToken_NewUserScopes(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	// Create a user in new-user group
	newUserGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.NewUserGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Verified User").
		SetEmail("verified8@example.com"). // Unique email
		SetGroup(newUserGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token
	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify token has new-user scopes
	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "me:*")
	assert.Equal(t, "login", tokenInfo.Meta["initiate_from_flow"])
	assert.Empty(t, tokenInfo.Meta["impersonation"])
}

func TestGrantToken_UserWithoutScopeSet(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	// Create a group without scope set
	groupWithoutScope, err := client.Group.Create().
		SetName("no-scope-group").
		SetDescription("Group without scope set").
		Save(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("No Scope User").
		SetEmail("noscope9@example.com"). // Unique email
		SetGroup(groupWithoutScope).
		Save(context)
	require.NoError(t, err)

	// Grant token - should succeed with empty scopes
	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Empty(t, tokenInfo.Scopes)
}

func TestRevokeToken_Success(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test10@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("registration"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	err = ctx.RevokeToken(context, token)
	require.NoError(t, err)

	_, err = authStorage.Get(context, token)
	require.Error(t, err)
	assert.Equal(t, auth.ErrNotFound, err)
}

func TestRevokeAllTokens_Success(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	ctx := useraccount.NewContext(client, authStorage)
	context := context.Background()

	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test11@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("registration"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	token2, err := ctx.GrantToken(
		context, user, "test-machine-2",
		useraccount.WithFlow("registration"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	err = ctx.RevokeAllTokens(context, user.ID)
	require.NoError(t, err)

	_, err = authStorage.Get(context, token)
	require.Error(t, err)
	assert.Equal(t, auth.ErrNotFound, err)

	_, err = authStorage.Get(context, token2)
	require.Error(t, err)
	assert.Equal(t, auth.ErrNotFound, err)
}
