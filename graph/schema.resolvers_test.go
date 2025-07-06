package graph

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/enttest"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

type mockAuthStorage struct {
	deleteByUserErr error
	createToken     string
	createErr       error
}

func (m *mockAuthStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	return m.createToken, m.createErr
}

func (m *mockAuthStorage) Delete(ctx context.Context, token string) error {
	panic("unimplemented")
}

func (m *mockAuthStorage) DeleteByUser(ctx context.Context, userID int) error {
	return m.deleteByUserErr
}

func (m *mockAuthStorage) Get(ctx context.Context, token string) (auth.TokenInfo, error) {
	panic("unimplemented")
}

func (m *mockAuthStorage) Peek(ctx context.Context, token string) (auth.TokenInfo, error) {
	panic("unimplemented")
}

func TestMutationResolver_LogoutAll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"me:write"},
		})

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response
		require.NoError(t, err)
		require.True(t, resp.LogoutAll)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation with no auth context
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrUnauthorized.Error())
	})

	t.Run("insufficient scope", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user but wrong scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"user:read"},
		})

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNoSufficientScope.Error())
	})

	t.Run("storage error", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Setup test resolver with mock auth storage
		storageErr := errors.New("storage error")
		resolver := &Resolver{
			ent: entClient,
			auth: &mockAuthStorage{
				deleteByUserErr: storageErr,
			},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"me:write"},
		})

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), storageErr.Error())
	})
}

func TestMutationResolver_ImpersonateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create a test user to impersonate
		userToImpersonate, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Setup test resolver with mock auth storage
		expectedToken := "test-token"
		resolver := &Resolver{
			ent: entClient,
			auth: &mockAuthStorage{
				createToken: expectedToken,
			},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"user:impersonate"},
		})

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err = c.Post(`mutation { impersonateUser(userID: `+strconv.Itoa(userToImpersonate.ID)+`) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, expectedToken, resp.ImpersonateUser)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation with no auth context
		var resp struct {
			ImpersonateUser string
		}
		err := c.Post(`mutation { impersonateUser(userID: 123) }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrUnauthorized.Error())
	})

	t.Run("insufficient scope", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user but wrong scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"user:read"},
		})

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err := c.Post(`mutation { impersonateUser(userID: 123) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNoSufficientScope.Error())
	})

	t.Run("storage error", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create a test user to impersonate
		userToImpersonate, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Setup test resolver with mock auth storage
		storageErr := errors.New("storage error")
		resolver := &Resolver{
			ent: entClient,
			auth: &mockAuthStorage{
				createErr: storageErr,
			},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"user:impersonate"},
		})

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err = c.Post(`mutation { impersonateUser(userID: `+strconv.Itoa(userToImpersonate.ID)+`) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), storageErr.Error())
	})
}

func createTestGroup(t *testing.T, client *ent.Client) (*ent.Group, error) {
	t.Helper()

	// Create a scopeset
	scopeset, err := client.ScopeSet.Create().
		SetSlug("test").
		SetScopes([]string{"*"}).
		Save(context.Background())
	if err != nil {
		return nil, err
	}

	// Create a group
	group, err := client.Group.Create().
		SetName("test").
		AddScopeSet(scopeset).
		Save(context.Background())
	if err != nil {
		return nil, err
	}

	return group, nil
}

func TestQueryResolver_Me(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup mock client that returns a user
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Setup test resolver
		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"me:read"},
		})

		// Execute query
		var resp struct {
			Me struct {
				Name  string
				Email string
			}
		}
		err = gqlClient.Post(`query { me { name email } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "testuser", resp.Me.Name)
		require.Equal(t, "test@example.com", resp.Me.Email)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query with no auth context
		var resp struct {
			Me struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { me { name } }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "UNAUTHORIZED")
	})

	t.Run("insufficient scope", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user but wrong scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 1,
			Scopes: []string{"me:write"},
		})

		// Execute query
		var resp struct {
			Me struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { me { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "FORBIDDEN")
	})

	t.Run("invalid user id", func(t *testing.T) {
		// Setup test resolver with real client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user but invalid user ID
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 0,
			Scopes: []string{"me:read"},
		})

		// Execute query
		var resp struct {
			Me struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { me { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "ent: user not found")
	})
}

func TestUserResolver_ImpersonatedBy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup mock client that returns a user
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create the impersonating user in the test database
		impersonator, err := entClient.User.Create().
			SetName("impersonator").
			SetEmail("impersonator@test.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create the impersonated user
		user, err := entClient.User.Create().
			SetName("impersonated").
			SetEmail("impersonated@test.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Setup test resolver
		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user and required scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"me:read", "user:impersonate"},
			Meta: map[string]string{
				"impersonated_by": strconv.Itoa(impersonator.ID),
			},
		})

		// Execute query
		var resp struct {
			Me struct {
				Name           string
				ImpersonatedBy struct {
					Name string
				}
			}
		}
		err = gqlClient.Post(`query { me { name impersonatedBy { name } } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "impersonated", resp.Me.Name)
		require.Equal(t, "impersonator", resp.Me.ImpersonatedBy.Name)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		// Setup test resolver with real ent client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query with no auth context
		var resp struct {
			Me struct {
				ImpersonatedBy struct {
					Name string
				}
			}
		}
		err := gqlClient.Post(`query { me { impersonatedBy { name } } }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "UNAUTHORIZED")
	})

	t.Run("insufficient scope", func(t *testing.T) {
		// Setup test resolver with real client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user but wrong scope
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"me:read"},
			Meta: map[string]string{
				"impersonated_by": "1",
			},
		})

		// Execute query
		var resp struct {
			Me struct {
				ImpersonatedBy struct {
					Name string
				}
			}
		}
		err = gqlClient.Post(`query { me { impersonatedBy { name } } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "FORBIDDEN")
	})

	t.Run("no impersonation metadata", func(t *testing.T) {
		// Setup test resolver with real client to avoid nil pointer
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create a test user
		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user and required scope but no impersonation metadata
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"me:read", "user:impersonate"},
			Meta:   map[string]string{},
		})

		// Execute query
		var resp struct {
			Me struct {
				Name           string
				ImpersonatedBy *struct {
					Name string
				}
			}
		}
		err = gqlClient.Post(`query { me { name impersonatedBy { name } } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "testuser", resp.Me.Name)
		require.Nil(t, resp.Me.ImpersonatedBy)
	})

	t.Run("impersonator not found", func(t *testing.T) {
		// Setup mock client that returns no user
		entClient := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
		defer func() {
			if err := entClient.Close(); err != nil {
				t.Fatalf("Failed to close client: %v", err)
			}
		}()

		// Create test group
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		// Create the impersonated user
		user, err := entClient.User.Create().
			SetName("impersonated").
			SetEmail("impersonated@test.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Setup test resolver
		resolver := &Resolver{
			ent:  entClient,
			auth: &mockAuthStorage{},
		}

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Create context with authenticated user, required scope and non-existent impersonator ID
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"me:read", "user:impersonate"},
			Meta: map[string]string{
				"impersonated_by": "999", // Non-existent user ID
			},
		})

		// Execute query
		var resp struct {
			Me struct {
				ImpersonatedBy *struct { // Make ImpersonatedBy a pointer to handle null
					Name string
				}
			}
		}
		err = gqlClient.Post(`query { me { impersonatedBy { name } } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify response - should return null for impersonatedBy without error
		require.NoError(t, err)
		require.Nil(t, resp.Me.ImpersonatedBy)
	})
}
