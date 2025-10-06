package useraccount_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// mockAuthStorage implements auth.Storage for testing
type mockAuthStorage struct {
	tokens map[string]auth.TokenInfo
}

func newMockAuthStorage() *mockAuthStorage {
	return &mockAuthStorage{
		tokens: make(map[string]auth.TokenInfo),
	}
}

func (m *mockAuthStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	token := "test-token-" + info.UserEmail
	m.tokens[token] = info
	return token, nil
}

func (m *mockAuthStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	info, exists := m.tokens[token]
	if !exists {
		return auth.TokenInfo{}, auth.ErrNotFound
	}
	return info, nil
}

func (m *mockAuthStorage) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	return m.Get(ctx, token)
}

func (m *mockAuthStorage) Delete(ctx context.Context, token string) error {
	delete(m.tokens, token)
	return nil
}

func (m *mockAuthStorage) DeleteByUser(ctx context.Context, userID int) error {
	for token, info := range m.tokens {
		if info.UserID == userID {
			delete(m.tokens, token)
		}
	}
	return nil
}

// setupTestDatabase creates a test database with required groups and scope sets
func setupTestDatabase(t *testing.T) *ent.Client {
	client := testhelper.NewEntSqliteClient(t)

	_, err := setup.Setup(context.Background(), client)
	require.NoError(t, err)

	return client
}

func TestNewContext(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)

	ctx := useraccount.NewContext(client, authStorage, eventService)
	require.NotNil(t, ctx)
}

func TestGetUser(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()
	eventService := events.NewEventService(client, nil)
	ctx := useraccount.NewContext(client, authStorage, eventService)
	context := context.Background()

	// Create a group for the user
	unverifiedGroup, err := client.Group.Query().Where(group.NameEQ("unverified")).Only(context)
	require.NoError(t, err)

	// Create a user
	userEnt, err := client.User.Create().
		SetName("Test User").
		SetEmail("test-getuser@example.com").
		SetGroup(unverifiedGroup).
		Save(context)
	require.NoError(t, err)

	// Test: Get existing user
	got, err := ctx.GetUser(context, userEnt.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, userEnt.ID, got.ID)
	require.Equal(t, userEnt.Email, got.Email)

	// Test: Get non-existent user
	_, err = ctx.GetUser(context, 999999)
	require.Error(t, err)
	require.ErrorIs(t, err, useraccount.ErrUserNotFound)
}
