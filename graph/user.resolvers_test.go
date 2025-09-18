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
	"github.com/database-playground/backend-v2/ent/group"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/submission"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/database-playground/backend-v2/internal/useraccount"
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

// NewTestResolver creates a resolver with all necessary dependencies for testing
func NewTestResolver(t *testing.T, entClient *ent.Client, authStorage auth.Storage) *Resolver {
	t.Helper()

	eventService := events.NewEventService(entClient)
	sqlrunner := testhelper.NewSQLRunnerClient(t)

	submissionService := submission.NewSubmissionService(entClient, eventService, sqlrunner)
	useraccountCtx := useraccount.NewContext(entClient, authStorage, eventService)

	return NewResolver(entClient, authStorage, sqlrunner, useraccountCtx, eventService, submissionService)
}

func TestMutationResolver_LogoutAll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.True(t, resp.LogoutAll)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.NewErrNoSufficientScope("me:write").Error())
	})

	t.Run("storage error", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Setup test resolver with mock auth storage
		storageErr := errors.New("storage error")
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{
			deleteByUserErr: storageErr,
		})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			LogoutAll bool
		}
		err := c.Post(`mutation { logoutAll }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), storageErr.Error())
	})
}

func TestMutationResolver_ImpersonateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

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
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{
			createToken: expectedToken,
		})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err = c.Post(`mutation { impersonateUser(userID: `+strconv.Itoa(userToImpersonate.ID)+`) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:impersonate"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, expectedToken, resp.ImpersonateUser)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err := c.Post(`mutation { impersonateUser(userID: 123) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.NewErrNoSufficientScope("user:impersonate").Error())
	})

	t.Run("storage error", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

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
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{
			createErr: storageErr,
		})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err = c.Post(`mutation { impersonateUser(userID: `+strconv.Itoa(userToImpersonate.ID)+`) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:impersonate"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), storageErr.Error())
	})

	t.Run("no such user", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			ImpersonateUser string
		}
		err := c.Post(`mutation { impersonateUser(userID: 123) }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:impersonate"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
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
		AddScopeSets(scopeset).
		Save(context.Background())
	if err != nil {
		return nil, err
	}

	return group, nil
}

func TestQueryResolver_Me(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup mock client that returns a user
		entClient := testhelper.NewEntSqliteClient(t)

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
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		var resp struct {
			Me struct {
				Name  string
				Email string
			}
		}
		err = gqlClient.Post(`query { me { name email } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user.ID,
					Scopes: []string{"me:read"},
				}),
			)
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "testuser", resp.Me.Name)
		require.Equal(t, "test@example.com", resp.Me.Email)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		var resp struct {
			Me struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { me { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("invalid user id", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		var resp struct {
			Me struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { me { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 0,
				Scopes: []string{"me:read"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "user not found")
	})
}

func TestQueryResolver_User(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)
	user, err := entClient.User.Create().
		SetName("user1").
		SetEmail("user1@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		var resp struct {
			User struct {
				Name  string
				Email string
			}
		}
		err := gqlClient.Post(`query { user(id: `+strconv.Itoa(user.ID)+`) { name email } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, "user1", resp.User.Name)
		require.Equal(t, "user1@example.com", resp.User.Email)
	})

	t.Run("not found", func(t *testing.T) {
		var resp struct {
			User struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { user(id: 99999) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("bad scope", func(t *testing.T) {
		var resp struct {
			User struct {
				Name string
			}
		}

		err := gqlClient.Post(`query { user(id: `+strconv.Itoa(user.ID)+`) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:write"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		var resp struct {
			User struct {
				Name string
			}
		}

		err := gqlClient.Post(`query { user(id: `+strconv.Itoa(user.ID)+`) { name } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQueryResolver_Group(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		var resp struct {
			Group struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { group(id: `+strconv.Itoa(group.ID)+`) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"group:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, "test", resp.Group.Name)
	})

	t.Run("not found", func(t *testing.T) {
		var resp struct {
			Group struct {
				Name string
			}
		}
		err := gqlClient.Post(`query { group(id: 99999) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"group:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("bad scope", func(t *testing.T) {
		var resp struct {
			Group struct {
				Name string
			}
		}

		err := gqlClient.Post(`query { group(id: `+strconv.Itoa(group.ID)+`) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"group:write"},
			}))
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		var resp struct {
			Group struct {
				Name string
			}
		}

		err := gqlClient.Post(`query { group(id: `+strconv.Itoa(group.ID)+`) { name } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQueryResolver_ScopeSet(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	scopeset, err := entClient.ScopeSet.Create().
		SetSlug("slug1").
		SetScopes([]string{"a", "b"}).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("by id", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}
		query := `query { scopeSet(filter: { id: ` + strconv.Itoa(scopeset.ID) + ` }) { slug } }`
		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"scopeset:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, "slug1", resp.ScopeSet.Slug)
	})

	t.Run("by slug", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}
		query := `query { scopeSet(filter: { slug: "slug1" }) { slug } }`
		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"scopeset:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, "slug1", resp.ScopeSet.Slug)
	})

	t.Run("no filter", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}
		query := `query { scopeSet(filter: { }) { slug } }`
		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"scopeset:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrInvalidFilter.Error())
	})

	t.Run("both filter", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}
		query := `query { scopeSet(filter: { id: 1, slug: "slug1" }) { slug } }`
		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"scopeset:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrInvalidFilter.Error())
	})

	t.Run("not found", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}
		query := `query { scopeSet(filter: { id: 99999 }) { slug } }`
		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"scopeset:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("bad scope", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}

		err := gqlClient.Post(`query { scopeSet(filter: { id: `+strconv.Itoa(scopeset.ID)+` }) { slug } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"unverified"},
			}))
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		var resp struct {
			ScopeSet struct {
				Slug string
			}
		}

		err := gqlClient.Post(`query { scopeSet(filter: { id: `+strconv.Itoa(scopeset.ID)+` }) { slug } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestUserResolver_ImpersonatedBy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

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
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

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
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"me:read", "user:impersonate"},
				Meta: map[string]string{
					useraccount.MetaImpersonation: strconv.Itoa(impersonator.ID),
				},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "impersonated", resp.Me.Name)
		require.Equal(t, "impersonator", resp.Me.ImpersonatedBy.Name)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("no impersonation metadata", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

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

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

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
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"me:read", "user:impersonate"},
				Meta:   map[string]string{},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "testuser", resp.Me.Name)
		require.Nil(t, resp.Me.ImpersonatedBy)
	})

	t.Run("impersonator not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

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
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		var resp struct {
			Me struct {
				ImpersonatedBy *struct { // Make ImpersonatedBy a pointer to handle null
					Name string
				}
			}
		}
		err = gqlClient.Post(`query { me { impersonatedBy { name } } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"me:read", "user:impersonate"},
				Meta: map[string]string{
					"impersonated_by": "999", // Non-existent user ID
				},
			}))
		})

		// Verify response - should return null for impersonatedBy without error
		require.NoError(t, err)
		require.Nil(t, resp.Me.ImpersonatedBy)
	})
}

func TestMutationResolver_UpdateMe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Create a test user
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}

		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute mutation
		var resp struct {
			UpdateMe struct {
				Name string
			}
		}
		err = gqlClient.Post(`mutation { updateMe(input: { name: "testuser" }) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.Equal(t, "testuser", resp.UpdateMe.Name)
	})

	t.Run("trying to update their group", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		var resp struct {
			UpdateMe struct {
				Name string
			}
		}
		err = gqlClient.Post(`mutation { updateMe(input: { groupID: "1" }) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrDisallowUpdateGroup.Error())
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute mutation with no auth context
		var resp struct {
			UpdateMe struct {
				Name string
			}
		}
		err := gqlClient.Post(`mutation { updateMe(input: { name: "testuser" }) { name } }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrUnauthorized.Error())
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}

		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute mutation
		var resp struct {
			UpdateMe struct {
				Name string
			}
		}
		err := gqlClient.Post(`mutation { updateMe(input: { name: "testuser" }) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"test"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.NewErrNoSufficientScope("me:write").Error())
	})

	t.Run("user not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}

		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute mutation
		var resp struct {
			UpdateMe struct {
				Name string
			}
		}
		err := gqlClient.Post(`mutation { updateMe(input: { name: "testuser" }) { name } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"me:write"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), useraccount.ErrUserNotFound.Error())
	})
}

func TestMutationResolver_VerifyRegistration(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Setup database with proper groups and scopes
		_, err := setup.Setup(context.Background(), entClient)
		require.NoError(t, err)

		// Get the unverified group
		unverifiedGroup, err := entClient.Group.Query().Where(group.NameEQ(useraccount.UnverifiedGroupSlug)).Only(context.Background())
		require.NoError(t, err)

		// Create an unverified user
		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(unverifiedGroup).
			Save(context.Background())
		require.NoError(t, err)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			VerifyRegistration bool
		}
		err = c.Post(`mutation { verifyRegistration }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user.ID,
				Scopes: []string{"verification:write"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		require.True(t, resp.VerifyRegistration)

		// Verify user was actually verified (moved to new-user group)
		updatedUser, err := entClient.User.Get(context.Background(), user.ID)
		require.NoError(t, err)

		updatedGroup, err := updatedUser.QueryGroup().Only(context.Background())
		require.NoError(t, err)
		require.Equal(t, useraccount.NewUserGroupSlug, updatedGroup.Name)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
			VerifyRegistration bool
		}
		err := c.Post(`mutation { verifyRegistration }`, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

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
			VerifyRegistration bool
		}
		err := c.Post(`mutation { verifyRegistration }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.NewErrNoSufficientScope("verification:write").Error())
	})

	t.Run("user already verified", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Setup database with proper groups and scopes
		_, err := setup.Setup(context.Background(), entClient)
		require.NoError(t, err)

		// Get the new-user group (verified users)
		newUserGroup, err := entClient.Group.Query().Where(group.NameEQ(useraccount.NewUserGroupSlug)).Only(context.Background())
		require.NoError(t, err)

		// Create a verified user (in new-user group)
		verifiedUser, err := entClient.User.Create().
			SetName("verifieduser").
			SetEmail("verified@example.com").
			SetGroup(newUserGroup).
			Save(context.Background())
		require.NoError(t, err)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Execute mutation
		var resp struct {
			VerifyRegistration bool
		}
		err = c.Post(`mutation { verifyRegistration }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: verifiedUser.ID,
				Scopes: []string{"verification:write"},
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrVerified.Error())
	})

	t.Run("user not found", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Setup database with proper groups and scopes
		_, err := setup.Setup(context.Background(), entClient)
		require.NoError(t, err)

		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server with scope directive
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		c := client.New(srv)

		// Create context with authenticated user but non-existent user ID
		ctx := auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: 999, // Non-existent user ID
			Scopes: []string{"verification:write"},
		})

		// Execute mutation
		var resp struct {
			VerifyRegistration bool
		}
		err = c.Post(`mutation { verifyRegistration }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(ctx)
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), "get user")
	})
}
