package graph

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestQueryResolver_Event(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	// Create test group and users
	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	ownerUser, err := entClient.User.Create().
		SetName("owner").
		SetEmail("owner@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	otherUser, err := entClient.User.Create().
		SetName("other").
		SetEmail("other@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	// Create test event owned by ownerUser
	event, err := entClient.Event.Create().
		SetType("login").
		SetTriggeredAt(time.Now()).
		SetUserID(ownerUser.ID).
		SetPayload(map[string]any{"machine": "test"}).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success - owner can access without scope", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID   string
				Type string
			}
		}
		err := gqlClient.Post(`query { event(id: `+strconv.Itoa(event.ID)+`) { id type } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{}, // No scopes
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(event.ID), resp.Event.ID)
		require.Equal(t, "login", resp.Event.Type)
	})

	t.Run("success - user with event:read scope can access", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID   string
				Type string
			}
		}
		err := gqlClient.Post(`query { event(id: `+strconv.Itoa(event.ID)+`) { id type } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"event:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(event.ID), resp.Event.ID)
		require.Equal(t, "login", resp.Event.Type)
	})

	t.Run("success - user with wildcard scope can access", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID   string
				Type string
			}
		}
		err := gqlClient.Post(`query { event(id: `+strconv.Itoa(event.ID)+`) { id type } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(event.ID), resp.Event.ID)
		require.Equal(t, "login", resp.Event.Type)
	})

	t.Run("forbidden - other user without scope cannot access", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { event(id: `+strconv.Itoa(event.ID)+`) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID,          // Different user
				Scopes: []string{"user:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrForbidden.Error())
	})

	t.Run("not found - non-existent event", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { event(id: 99999) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{"event:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			Event struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { event(id: `+strconv.Itoa(event.ID)+`) { id } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQueryResolver_PointGrant(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	// Create test group and users
	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	ownerUser, err := entClient.User.Create().
		SetName("owner").
		SetEmail("owner@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	otherUser, err := entClient.User.Create().
		SetName("other").
		SetEmail("other@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	// Create test point grant for ownerUser
	point, err := entClient.Point.Create().
		SetUser(ownerUser).
		SetPoints(100).
		SetDescription("Login bonus").
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success - owner can access without scope", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID          string
				Points      int
				Description string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: `+strconv.Itoa(point.ID)+`) { id points description } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{}, // No scopes
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(point.ID), resp.PointGrant.ID)
		require.Equal(t, 100, resp.PointGrant.Points)
		require.Equal(t, "Login bonus", resp.PointGrant.Description)
	})

	t.Run("success - user with point:read scope can access", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID          string
				Points      int
				Description string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: `+strconv.Itoa(point.ID)+`) { id points description } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"point:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(point.ID), resp.PointGrant.ID)
		require.Equal(t, 100, resp.PointGrant.Points)
		require.Equal(t, "Login bonus", resp.PointGrant.Description)
	})

	t.Run("success - user with wildcard scope can access", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID          string
				Points      int
				Description string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: `+strconv.Itoa(point.ID)+`) { id points description } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(point.ID), resp.PointGrant.ID)
		require.Equal(t, 100, resp.PointGrant.Points)
		require.Equal(t, "Login bonus", resp.PointGrant.Description)
	})

	t.Run("forbidden - other user without scope cannot access", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: `+strconv.Itoa(point.ID)+`) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID,          // Different user
				Scopes: []string{"user:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrForbidden.Error())
	})

	t.Run("not found - non-existent point grant", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: 99999) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{"point:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			PointGrant struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { pointGrant(id: `+strconv.Itoa(point.ID)+`) { id } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}
