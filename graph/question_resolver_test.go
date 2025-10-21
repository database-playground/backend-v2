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
	// Generate unique slug and relation figure to avoid UNIQUE constraint violations
	uniqueID := strconv.FormatInt(time.Now().UnixNano(), 10)
	database, err := entClient.Database.Create().
		SetSlug("test-db-" + uniqueID).
		SetDescription("Test Database").
		SetSchema("CREATE TABLE test (id INT, name VARCHAR(255));").
		SetRelationFigure("test-relation-figure-" + uniqueID).
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

func TestQueryResolver_QuestionCategories(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	// Create test group and user
	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	testUser, err := entClient.User.Create().
		SetName("testUser").
		SetEmail("test@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success - returns empty array when no questions", func(t *testing.T) {
		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.NotNil(t, resp.QuestionCategories)
		require.Len(t, resp.QuestionCategories, 0)
	})

	t.Run("success - returns single category", func(t *testing.T) {
		// Create test database
		database := createTestDatabase(t, entClient)

		// Create question with category
		_, err := entClient.Question.Create().
			SetCategory("basic-select").
			SetDifficulty("easy").
			SetTitle("Test Query 1").
			SetDescription("Write a SELECT query").
			SetReferenceAnswer("SELECT * FROM test;").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Len(t, resp.QuestionCategories, 1)
		require.Contains(t, resp.QuestionCategories, "basic-select")
	})

	t.Run("success - returns unique categories when questions have duplicate categories", func(t *testing.T) {
		// Create test database
		database := createTestDatabase(t, entClient)

		// Create multiple questions with same category
		_, err := entClient.Question.Create().
			SetCategory("joins").
			SetDifficulty("medium").
			SetTitle("Join Query 1").
			SetDescription("Write a JOIN query").
			SetReferenceAnswer("SELECT * FROM test t1 JOIN test t2 ON t1.id = t2.id;").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		_, err = entClient.Question.Create().
			SetCategory("joins").
			SetDifficulty("medium").
			SetTitle("Join Query 2").
			SetDescription("Write another JOIN query").
			SetReferenceAnswer("SELECT * FROM test t1 LEFT JOIN test t2 ON t1.id = t2.id;").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		// Should have at least one category (joins)
		require.GreaterOrEqual(t, len(resp.QuestionCategories), 1)
		require.Contains(t, resp.QuestionCategories, "joins")

		// Count occurrences of "joins" - should only appear once
		joinCount := 0
		for _, cat := range resp.QuestionCategories {
			if cat == "joins" {
				joinCount++
			}
		}
		require.Equal(t, 1, joinCount, "Category 'joins' should appear exactly once")
	})

	t.Run("success - returns multiple different categories", func(t *testing.T) {
		// Create test database
		database := createTestDatabase(t, entClient)

		// Create questions with different categories
		_, err := entClient.Question.Create().
			SetCategory("aggregation").
			SetDifficulty("easy").
			SetTitle("Aggregation Query").
			SetDescription("Use aggregation functions").
			SetReferenceAnswer("SELECT COUNT(*) FROM test;").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		_, err = entClient.Question.Create().
			SetCategory("subqueries").
			SetDifficulty("hard").
			SetTitle("Subquery Challenge").
			SetDescription("Use subqueries").
			SetReferenceAnswer("SELECT * FROM test WHERE id IN (SELECT id FROM test);").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(resp.QuestionCategories), 2)
		require.Contains(t, resp.QuestionCategories, "aggregation")
		require.Contains(t, resp.QuestionCategories, "subqueries")
	})

	t.Run("success - works with wildcard scope", func(t *testing.T) {
		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.NotNil(t, resp.QuestionCategories)
		// At this point we should have at least categories from previous tests in this run
		// Since we don't clean up between tests in the same test function, we expect at least 1
		require.GreaterOrEqual(t, len(resp.QuestionCategories), 1)
	})

	t.Run("forbidden - user without question:read scope", func(t *testing.T) {
		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"submission:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		var resp struct {
			QuestionCategories []string
		}
		query := `query { questionCategories }`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}

func TestQuestionResolver_Statistics(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)
	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	// Create test group
	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	// Create test user for authentication
	testUser, err := entClient.User.Create().
		SetName("testUser").
		SetEmail("test@example.com").
		SetGroup(group).
		Save(context.Background())
	require.NoError(t, err)

	t.Run("success - no submissions returns all zeros", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 0, resp.Question.Statistics.CorrectSubmissionCount)
		require.Equal(t, 0, resp.Question.Statistics.SubmissionCount)
		require.Equal(t, 0, resp.Question.Statistics.AttemptedUsers)
		require.Equal(t, 0, resp.Question.Statistics.PassedUsers)
	})

	t.Run("success - single user with single successful submission", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create user
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create successful submission
		createTestSubmission(t, entClient, user1, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 1, resp.Question.Statistics.CorrectSubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.SubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.AttemptedUsers)
		require.Equal(t, 1, resp.Question.Statistics.PassedUsers)
	})

	t.Run("success - single user with single failed submission", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create user
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1_failed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create failed submission
		createTestSubmission(t, entClient, user1, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 0, resp.Question.Statistics.CorrectSubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.SubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.AttemptedUsers)
		require.Equal(t, 0, resp.Question.Statistics.PassedUsers)
	})

	t.Run("success - single user with multiple submissions (mixed success and failure)", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create user
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1_mixed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create multiple submissions
		createTestSubmission(t, entClient, user1, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-3*time.Hour))
		createTestSubmission(t, entClient, user1, question, "SELECT id FROM test;", submission.StatusFailed, time.Now().Add(-2*time.Hour))
		createTestSubmission(t, entClient, user1, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, user1, question, "SELECT name FROM test;", submission.StatusSuccess, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 2, resp.Question.Statistics.CorrectSubmissionCount, "should count 2 successful submissions")
		require.Equal(t, 4, resp.Question.Statistics.SubmissionCount, "should count all 4 submissions")
		require.Equal(t, 1, resp.Question.Statistics.AttemptedUsers, "should count only 1 unique user")
		require.Equal(t, 1, resp.Question.Statistics.PassedUsers, "should count 1 user who passed")
	})

	t.Run("success - multiple users with different submission statuses", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create users
		userPassed1, err := entClient.User.Create().
			SetName("userPassed1").
			SetEmail("userPassed1@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		userPassed2, err := entClient.User.Create().
			SetName("userPassed2").
			SetEmail("userPassed2@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		userFailed, err := entClient.User.Create().
			SetName("userFailed").
			SetEmail("userFailed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		userMixed, err := entClient.User.Create().
			SetName("userMixed").
			SetEmail("userMixed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// userPassed1: 1 successful submission
		createTestSubmission(t, entClient, userPassed1, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// userPassed2: 2 successful submissions
		createTestSubmission(t, entClient, userPassed2, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, userPassed2, question, "SELECT id FROM test;", submission.StatusSuccess, time.Now())

		// userFailed: 2 failed submissions
		createTestSubmission(t, entClient, userFailed, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, userFailed, question, "INVALID SQL;", submission.StatusFailed, time.Now())

		// userMixed: 1 failed, 1 successful
		createTestSubmission(t, entClient, userMixed, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, userMixed, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 4, resp.Question.Statistics.CorrectSubmissionCount, "should count 4 successful submissions (1+2+0+1)")
		require.Equal(t, 7, resp.Question.Statistics.SubmissionCount, "should count all 7 submissions")
		require.Equal(t, 4, resp.Question.Statistics.AttemptedUsers, "should count 4 unique users")
		require.Equal(t, 3, resp.Question.Statistics.PassedUsers, "should count 3 users who passed (userPassed1, userPassed2, userMixed)")
	})

	t.Run("success - multiple users attempted but none passed", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create users with only failed submissions
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1_nopassed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		user2, err := entClient.User.Create().
			SetName("user2").
			SetEmail("user2_nopassed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		user3, err := entClient.User.Create().
			SetName("user3").
			SetEmail("user3_nopassed@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// All users have failed submissions
		createTestSubmission(t, entClient, user1, question, "INVALID SQL;", submission.StatusFailed, time.Now())
		createTestSubmission(t, entClient, user2, question, "SELECT * FROM invalid;", submission.StatusFailed, time.Now())
		createTestSubmission(t, entClient, user3, question, "WRONG QUERY;", submission.StatusFailed, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 0, resp.Question.Statistics.CorrectSubmissionCount)
		require.Equal(t, 3, resp.Question.Statistics.SubmissionCount)
		require.Equal(t, 3, resp.Question.Statistics.AttemptedUsers)
		require.Equal(t, 0, resp.Question.Statistics.PassedUsers)
	})

	t.Run("success - works with wildcard scope", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		// Create user and submission
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1_wildcard@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		createTestSubmission(t, entClient, user1, question, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"*"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 1, resp.Question.Statistics.CorrectSubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.SubmissionCount)
		require.Equal(t, 1, resp.Question.Statistics.AttemptedUsers)
		require.Equal(t, 1, resp.Question.Statistics.PassedUsers)
	})

	t.Run("success - statistics are isolated per question", func(t *testing.T) {
		// Create test database
		database := createTestDatabase(t, entClient)

		// Create two different questions
		question1 := createTestQuestion(t, entClient, database)
		question2 := createTestQuestion(t, entClient, database)

		// Create user
		user1, err := entClient.User.Create().
			SetName("user1").
			SetEmail("user1_isolated@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create submissions for question1
		createTestSubmission(t, entClient, user1, question1, "SELECT * FROM test;", submission.StatusSuccess, time.Now())
		createTestSubmission(t, entClient, user1, question1, "SELECT id FROM test;", submission.StatusSuccess, time.Now().Add(1*time.Hour))

		// Create submissions for question2
		createTestSubmission(t, entClient, user1, question2, "SELECT * FROM invalid;", submission.StatusFailed, time.Now())

		// Query question1 statistics
		var resp1 struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query1 := `query { question(id: ` + strconv.Itoa(question1.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query1, &resp1, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 2, resp1.Question.Statistics.CorrectSubmissionCount, "question1 should have 2 correct submissions")
		require.Equal(t, 2, resp1.Question.Statistics.SubmissionCount, "question1 should have 2 total submissions")
		require.Equal(t, 1, resp1.Question.Statistics.AttemptedUsers, "question1 should have 1 attempted user")
		require.Equal(t, 1, resp1.Question.Statistics.PassedUsers, "question1 should have 1 passed user")

		// Query question2 statistics
		var resp2 struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
					SubmissionCount        int
					AttemptedUsers         int
					PassedUsers            int
				}
			}
		}
		query2 := `query { question(id: ` + strconv.Itoa(question2.ID) + `) { 
			statistics { 
				correctSubmissionCount 
				submissionCount 
				attemptedUsers 
				passedUsers 
			} 
		} }`

		err = gqlClient.Post(query2, &resp2, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"question:read"},
			}))
		})
		require.NoError(t, err)
		require.Equal(t, 0, resp2.Question.Statistics.CorrectSubmissionCount, "question2 should have 0 correct submissions")
		require.Equal(t, 1, resp2.Question.Statistics.SubmissionCount, "question2 should have 1 total submission")
		require.Equal(t, 1, resp2.Question.Statistics.AttemptedUsers, "question2 should have 1 attempted user")
		require.Equal(t, 0, resp2.Question.Statistics.PassedUsers, "question2 should have 0 passed users")
	})

	t.Run("forbidden - user without question:read scope", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
			} 
		} }`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: testUser.ID,
				Scopes: []string{"submission:read"}, // Wrong scope
			}))
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		// Create test database and question
		database := createTestDatabase(t, entClient)
		question := createTestQuestion(t, entClient, database)

		var resp struct {
			Question struct {
				Statistics struct {
					CorrectSubmissionCount int
				}
			}
		}
		query := `query { question(id: ` + strconv.Itoa(question.ID) + `) { 
			statistics { 
				correctSubmissionCount 
			} 
		} }`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})
}
