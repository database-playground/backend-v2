package useraccount_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/enttest"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/setup"
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
	// Use a unique database for each test to avoid conflicts
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=private&_fk=1")
	ctx := context.Background()

	_, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	return client
}

func TestNewContext(t *testing.T) {
	client := setupTestDatabase(t)
	authStorage := newMockAuthStorage()

	ctx := useraccount.NewContext(client, authStorage)
	require.NotNil(t, ctx)
}
