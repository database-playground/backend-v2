package graph

import (
	"context"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/stretchr/testify/require"
)

type mockAuthStorage struct {
	deleteByUserErr error
}

func (m *mockAuthStorage) Create(ctx context.Context, info auth.TokenInfo) (string, error) {
	panic("unimplemented")
}

func (m *mockAuthStorage) Delete(ctx context.Context, token string) error {
	panic("unimplemented")
}

func (m *mockAuthStorage) DeleteByUser(ctx context.Context, user string) error {
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
		// Setup test resolver with mock auth storage
		resolver := &Resolver{
			ent:  &ent.Client{},
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
			User:   "testuser",
			Scopes: []string{"user:write"},
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
		// Setup test resolver with mock auth storage
		resolver := &Resolver{
			ent:  &ent.Client{},
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
		// Setup test resolver with mock auth storage
		resolver := &Resolver{
			ent:  &ent.Client{},
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
			User:   "testuser",
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
		// Setup test resolver with mock auth storage
		storageErr := errors.New("storage error")
		resolver := &Resolver{
			ent: &ent.Client{},
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
			User:   "testuser",
			Scopes: []string{"user:write"},
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
