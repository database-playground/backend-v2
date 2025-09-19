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

func TestQuestionResolver_UserSubmissions(t *testing.T) {
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

	user1, err := entClient.User.Create().
		SetName("user1").
		SetEmail("user1@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	user2, err := entClient.User.Create().
		SetName("user2").
		SetEmail("user2@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	// Create test database and question
	database := createTestDatabase(t, entClient)
	question := createTestQuestion(t, entClient, database)

	// Create multiple submissions for user1
	submission1 := createTestSubmission(t, entClient, user1, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-2*time.Hour))
	submission2 := createTestSubmission(t, entClient, user1, question, "SELECT id FROM test;", submission.StatusFailed, time.Now().Add(-1*time.Hour))
	submission3 := createTestSubmission(t, entClient, user1, question, "SELECT name FROM test;", submission.StatusSuccess, time.Now())

	// Create submission for user2 (should not appear in user1's results)
	createTestSubmission(t, entClient, user2, question, "SELECT count(*) FROM test;", submission.StatusSuccess, time.Now())

	t.Run("success - returns user submissions with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				UserSubmissions []struct {
					ID            string
					SubmittedCode string
					Status        string
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { userSubmissions { id submittedCode status } } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user1.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)

		// Should return 3 submissions for user1
		require.Len(t, resp.Question.UserSubmissions, 3)

		// Check that all submissions belong to the authenticated user
		submissionIDs := make(map[string]bool)
		for _, sub := range resp.Question.UserSubmissions {
			submissionIDs[sub.ID] = true
		}
		require.True(t, submissionIDs[strconv.Itoa(submission1.ID)])
		require.True(t, submissionIDs[strconv.Itoa(submission2.ID)])
		require.True(t, submissionIDs[strconv.Itoa(submission3.ID)])
	})

	t.Run("success - returns user submissions with wildcard scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				UserSubmissions []struct {
					ID            string
					SubmittedCode string
					Status        string
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { userSubmissions { id submittedCode status } } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user1.ID,
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)

		// Should return 3 submissions for user1
		require.Len(t, resp.Question.UserSubmissions, 3)
	})

	t.Run("success - no submissions returns empty array", func(t *testing.T) {
		// Create a user with no submissions
		userNoSubs, err := entClient.User.Create().
			SetName("userNoSubs").
			SetEmail("userNoSubs@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		var resp struct {
			Question struct {
				UserSubmissions []struct {
					ID string
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { userSubmissions { id } } }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userNoSubs.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Len(t, resp.Question.UserSubmissions, 0)
	})

	t.Run("forbidden - user without question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				UserSubmissions []struct {
					ID string
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { userSubmissions { id } } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: user1.ID,
				Scopes: []string{"submission:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			Question struct {
				UserSubmissions []struct {
					ID string
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { userSubmissions { id } } }`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQuestionResolver_Attempted(t *testing.T) {
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

	userWithSubmissions, err := entClient.User.Create().
		SetName("userWithSubmissions").
		SetEmail("userWithSubmissions@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	userWithoutSubmissions, err := entClient.User.Create().
		SetName("userWithoutSubmissions").
		SetEmail("userWithoutSubmissions@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	// Create test database and question
	database := createTestDatabase(t, entClient)
	question := createTestQuestion(t, entClient, database)

	// Create submission for userWithSubmissions
	createTestSubmission(t, entClient, userWithSubmissions, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

	t.Run("success - user has attempted with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Attempted bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { attempted } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userWithSubmissions.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.True(t, resp.Question.Attempted)
	})

	t.Run("success - user has not attempted with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Attempted bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { attempted } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userWithoutSubmissions.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.False(t, resp.Question.Attempted)
	})

	t.Run("success - user has attempted with wildcard scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Attempted bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { attempted } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userWithSubmissions.ID,
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.True(t, resp.Question.Attempted)
	})

	t.Run("forbidden - user without question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Attempted bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { attempted } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userWithSubmissions.ID,
				Scopes: []string{"submission:write"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			Question struct {
				Attempted bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { attempted } }`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQuestionResolver_Solved(t *testing.T) {
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

	userSolved, err := entClient.User.Create().
		SetName("userSolved").
		SetEmail("userSolved@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	userNotSolved, err := entClient.User.Create().
		SetName("userNotSolved").
		SetEmail("userNotSolved@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	userNoSubmissions, err := entClient.User.Create().
		SetName("userNoSubmissions").
		SetEmail("userNoSubmissions@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	// Create test database and question
	database := createTestDatabase(t, entClient)
	question := createTestQuestion(t, entClient, database)

	// Create successful submission for userSolved
	createTestSubmission(t, entClient, userSolved, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

	// Create failed submission for userNotSolved
	createTestSubmission(t, entClient, userNotSolved, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now())

	t.Run("success - user has solved with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userSolved.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.True(t, resp.Question.Solved)
	})

	t.Run("success - user has not solved (failed submissions only) with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userNotSolved.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.False(t, resp.Question.Solved)
	})

	t.Run("success - user has not solved (no submissions) with question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userNoSubmissions.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.False(t, resp.Question.Solved)
	})

	t.Run("success - user solved after failed attempts with question:read scope", func(t *testing.T) {
		// Create a user with both failed and successful submissions
		userMixed, err := entClient.User.Create().
			SetName("userMixed").
			SetEmail("userMixed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create failed submission first
		createTestSubmission(t, entClient, userMixed, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-1*time.Hour))
		// Create successful submission later
		createTestSubmission(t, entClient, userMixed, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userMixed.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.True(t, resp.Question.Solved)
	})

	t.Run("success - user has solved with wildcard scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userSolved.ID,
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.True(t, resp.Question.Solved)
	})

	t.Run("forbidden - user without question:read scope", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: userSolved.ID,
				Scopes: []string{"user:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			Question struct {
				Solved bool
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { solved } }`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}
