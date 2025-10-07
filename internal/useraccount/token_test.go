package useraccount_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/ent/event"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/auth"
	events_pkg "github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantToken_Success(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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
	assert.Equal(t, "registration", tokenInfo.Meta[useraccount.MetaInitiateFromFlow])
	assert.Empty(t, tokenInfo.Meta[useraccount.MetaImpersonation])
}

func TestGrantToken_Impersonation(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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
	assert.Equal(t, "registration", tokenInfo.Meta[useraccount.MetaInitiateFromFlow])
	assert.Equal(t, strconv.Itoa(user.ID), tokenInfo.Meta[useraccount.MetaImpersonation])
}

func TestGrantToken_DefaultFlow(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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
	assert.Equal(t, "undefined", tokenInfo.Meta[useraccount.MetaInitiateFromFlow])
	assert.Empty(t, tokenInfo.Meta[useraccount.MetaImpersonation])
}

func TestGrantToken_NewUserScopes(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in student group
	studentGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.StudentGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Verified User").
		SetEmail("verified8@example.com"). // Unique email
		SetGroup(studentGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token
	token, err := ctx.GrantToken(
		context, user, "test-machine",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify token has student scopes
	tokenInfo, err := authStorage.Get(context, token)
	require.NoError(t, err)
	assert.Contains(t, tokenInfo.Scopes, "me:*")
	assert.Equal(t, "login", tokenInfo.Meta[useraccount.MetaInitiateFromFlow])
	assert.Empty(t, tokenInfo.Meta[useraccount.MetaImpersonation])
}

func TestGrantToken_UserWithoutScopeSet(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
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

func TestGrantToken_LoginEventTriggered(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-event-login@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token (should trigger login event)
	token, err := ctx.GrantToken(
		context, user, "test-machine-login",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify login event was created in database
	loginEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeLogin))).
		All(context)
	require.NoError(t, err)
	require.Len(t, loginEvents, 1)

	// Verify event payload contains correct machine info
	loginEvent := loginEvents[0]
	assert.Equal(t, user.ID, loginEvent.UserID)
	assert.Equal(t, string(events_pkg.EventTypeLogin), loginEvent.Type)
	assert.NotNil(t, loginEvent.Payload)
	assert.Equal(t, "test-machine-login", loginEvent.Payload["machine"])
	assert.NotZero(t, loginEvent.TriggeredAt)
}

func TestGrantToken_ImpersonationEventTriggered(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-event-impersonation@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Create an impersonator user
	impersonator, err := client.User.Create().
		SetName("Impersonator User").
		SetEmail("impersonator@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token with impersonation (should trigger impersonation event)
	token, err := ctx.GrantToken(
		context, user, "test-machine-impersonation",
		useraccount.WithFlow("admin"),
		useraccount.WithImpersonation(impersonator.ID),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify impersonation event was created in database
	impersonationEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeImpersonated))).
		All(context)
	require.NoError(t, err)
	require.Len(t, impersonationEvents, 1)

	// Verify event payload contains correct impersonator info
	impersonationEvent := impersonationEvents[0]
	assert.Equal(t, user.ID, impersonationEvent.UserID)
	assert.Equal(t, string(events_pkg.EventTypeImpersonated), impersonationEvent.Type)
	assert.NotNil(t, impersonationEvent.Payload)

	// JSON unmarshaling converts numbers to float64, so we need to convert
	impersonatorIDFloat, ok := impersonationEvent.Payload["impersonator_id"].(float64)
	require.True(t, ok, "impersonator_id should be a number")
	assert.Equal(t, float64(impersonator.ID), impersonatorIDFloat)
	assert.NotZero(t, impersonationEvent.TriggeredAt)

	// Verify no login event was created (impersonation takes precedence)
	loginEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeLogin))).
		All(context)
	require.NoError(t, err)
	require.Len(t, loginEvents, 0)
}

func TestGrantToken_MultipleTokensCreateMultipleEvents(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-event-multiple@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant multiple tokens (should create multiple login events)
	// Note: We'll grant them sequentially to avoid race conditions
	token1, err := ctx.GrantToken(
		context, user, "machine-1",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token1)

	// Wait for the first event to be processed
	time.Sleep(50 * time.Millisecond)

	token2, err := ctx.GrantToken(
		context, user, "machine-2",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	// Wait for the second event to be processed
	time.Sleep(50 * time.Millisecond)

	token3, err := ctx.GrantToken(
		context, user, "machine-3",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token3)

	// Verify three login events were created
	loginEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeLogin))).
		All(context)
	require.NoError(t, err)
	require.Len(t, loginEvents, 3)

	// Verify each event has different machine info
	machines := make(map[string]bool)
	for _, event := range loginEvents {
		machine, ok := event.Payload["machine"].(string)
		require.True(t, ok, "machine should be a string")
		machines[machine] = true
	}
	assert.Len(t, machines, 3)
	assert.True(t, machines["machine-1"])
	assert.True(t, machines["machine-2"])
	assert.True(t, machines["machine-3"])
}

func TestRevokeToken_LogoutEventTriggered(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-event-logout@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant token first
	token, err := ctx.GrantToken(
		context, user, "test-machine-logout",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Revoke token (should trigger logout event)
	err = ctx.RevokeToken(context, token)
	require.NoError(t, err)

	// Verify logout event was created in database
	logoutEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeLogout))).
		All(context)
	require.NoError(t, err)
	require.Len(t, logoutEvents, 1)

	// Verify event details
	logoutEvent := logoutEvents[0]
	assert.Equal(t, user.ID, logoutEvent.UserID)
	assert.Equal(t, string(events_pkg.EventTypeLogout), logoutEvent.Type)
	assert.NotZero(t, logoutEvent.TriggeredAt)
}

func TestRevokeAllTokens_LogoutAllEventTriggered(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events_pkg.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a user in unverified group
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-event-logout-all@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Grant multiple tokens first
	token1, err := ctx.GrantToken(
		context, user, "test-machine-1",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token1)

	token2, err := ctx.GrantToken(
		context, user, "test-machine-2",
		useraccount.WithFlow("login"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	// Revoke all tokens (should trigger logout_all event)
	err = ctx.RevokeAllTokens(context, user.ID)
	require.NoError(t, err)

	// Verify logout_all event was created in database
	logoutAllEvents, err := client.Event.Query().
		Where(event.UserIDEQ(user.ID)).
		Where(event.TypeEQ(string(events_pkg.EventTypeLogoutAll))).
		All(context)
	require.NoError(t, err)
	require.Len(t, logoutAllEvents, 1)

	// Verify event details
	logoutAllEvent := logoutAllEvents[0]
	assert.Equal(t, user.ID, logoutAllEvent.UserID)
	assert.Equal(t, string(events_pkg.EventTypeLogoutAll), logoutAllEvent.Type)
	assert.NotZero(t, logoutAllEvent.TriggeredAt)

	// Verify tokens are actually revoked
	_, err = authStorage.Get(context, token1)
	require.Error(t, err)
	assert.Equal(t, auth.ErrNotFound, err)

	_, err = authStorage.Get(context, token2)
	require.Error(t, err)
	assert.Equal(t, auth.ErrNotFound, err)
}
