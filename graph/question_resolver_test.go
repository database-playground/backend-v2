package graph

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// createTestDatabase creates a test database entity
func createTestDatabase(t *testing.T, entClient *ent.Client) *ent.Database {
	t.Helper()
	database, err := entClient.Database.Create().
		SetSlug("test-db").
		SetDescription("Test Database").
		SetSchema("CREATE TABLE test (id INT, name VARCHAR(255));").
		SetRelationFigure("test-relation-figure").
		Save(context.Background())
	require.NoError(t, err)
	return database
}

// createTestQuestion creates a test question entity
func createTestQuestion(t *testing.T, entClient *ent.Client, database *ent.Database) *ent.Question {
	t.Helper()
	question, err := entClient.Question.Create().
		SetCategory("test-query").
		SetDifficulty("easy").
		SetTitle("Test Query").
		SetDescription("Write a SELECT query").
		SetReferenceAnswer("SELECT * FROM test;").
		SetDatabase(database).
		Save(context.Background())
	require.NoError(t, err)
	return question
}

// createTestSubmission creates a test submission entity
func createTestSubmission(t *testing.T, entClient *ent.Client, user *ent.User, question *ent.Question, code string, status submission.Status, submittedAt time.Time) *ent.Submission {
	t.Helper()
	submission, err := entClient.Submission.Create().
		SetSubmittedCode(code).
		SetStatus(status).
		SetSubmittedAt(submittedAt).
		SetUser(user).
		SetQuestion(question).
		Save(context.Background())
	require.NoError(t, err)
	return submission
}

func TestQueryResolver_Submission(t *testing.T) {
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

	// Create test database and question
	database, err := entClient.Database.Create().
		SetSlug("test-db").
		SetDescription("Test Database").
		SetSchema("CREATE TABLE test (id INT, name VARCHAR(255));").
		SetRelationFigure("test-relation-figure").
		Save(context.Background())
	require.NoError(t, err)

	question, err := entClient.Question.Create().
		SetCategory("test-query").
		SetDifficulty("easy").
		SetTitle("Test Query").
		SetDescription("Write a SELECT query").
		SetReferenceAnswer("SELECT * FROM test;").
		SetDatabase(database).
		Save(context.Background())
	require.NoError(t, err)

	// Create test submission for ownerUser
	submission, err := entClient.Submission.Create().
		SetSubmittedCode("SELECT * FROM test;").
		SetStatus("success").
		SetSubmittedAt(time.Now()).
		SetUser(ownerUser).
		SetQuestion(question).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success - owner can access without scope", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID            string
				SubmittedCode string
				Status        string
			}
		}
		err := gqlClient.Post(`query { submission(id: `+strconv.Itoa(submission.ID)+`) { id submittedCode status } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{}, // No scopes
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(submission.ID), resp.Submission.ID)
		require.Equal(t, "SELECT * FROM test;", resp.Submission.SubmittedCode)
		require.Equal(t, "success", resp.Submission.Status)
	})

	t.Run("success - user with submission:read scope can access", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID            string
				SubmittedCode string
				Status        string
			}
		}
		err := gqlClient.Post(`query { submission(id: `+strconv.Itoa(submission.ID)+`) { id submittedCode status } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"submission:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(submission.ID), resp.Submission.ID)
		require.Equal(t, "SELECT * FROM test;", resp.Submission.SubmittedCode)
		require.Equal(t, "success", resp.Submission.Status)
	})

	t.Run("success - user with wildcard scope can access", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID            string
				SubmittedCode string
				Status        string
			}
		}
		err := gqlClient.Post(`query { submission(id: `+strconv.Itoa(submission.ID)+`) { id submittedCode status } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID, // Different user
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(submission.ID), resp.Submission.ID)
		require.Equal(t, "SELECT * FROM test;", resp.Submission.SubmittedCode)
		require.Equal(t, "success", resp.Submission.Status)
	})

	t.Run("forbidden - other user without scope cannot access", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { submission(id: `+strconv.Itoa(submission.ID)+`) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: otherUser.ID,          // Different user
				Scopes: []string{"user:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrForbidden.Error())
	})

	t.Run("not found - non-existent submission", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { submission(id: 99999) { id } }`, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: ownerUser.ID,
				Scopes: []string{"submission:read"},
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.ErrNotFound.Error())
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			Submission struct {
				ID string
			}
		}
		err := gqlClient.Post(`query { submission(id: `+strconv.Itoa(submission.ID)+`) { id } }`, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}
