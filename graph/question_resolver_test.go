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
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/setup"
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

func TestUserResolver_SubmissionsOfQuestion(t *testing.T) {
	entClient := testhelper.NewEntSqliteClient(t)

	// Setup database with required groups and scope sets
	setupResult, err := setup.Setup(context.Background(), entClient)
	require.NoError(t, err)

	// Use the existing new-user group from setup
	testGroup := setupResult.NewUserGroup

	// Create test users
	user1, err := entClient.User.Create().
		SetName("testuser1").
		SetEmail("test1@example.com").
		SetGroup(testGroup).
		Save(context.Background())
	require.NoError(t, err)

	user2, err := entClient.User.Create().
		SetName("testuser2").
		SetEmail("test2@example.com").
		SetGroup(testGroup).
		Save(context.Background())
	require.NoError(t, err)

	// Create test database and question
	database := createTestDatabase(t, entClient)
	question := createTestQuestion(t, entClient, database)

	// Create test submissions for user1
	now := time.Now()
	submissions := []*ent.Submission{
		createTestSubmission(t, entClient, user1, question, "SELECT * FROM test;", submission.StatusSuccess, now.Add(-3*time.Hour)),
		createTestSubmission(t, entClient, user1, question, "SELECT id FROM test;", submission.StatusSuccess, now.Add(-2*time.Hour)),
		createTestSubmission(t, entClient, user1, question, "INVALID SQL;", submission.StatusFailed, now.Add(-1*time.Hour)),
		createTestSubmission(t, entClient, user1, question, "SELECT name FROM test;", submission.StatusPending, now),
	}

	// Create submissions for user2 (to ensure they don't appear in user1's results)
	createTestSubmission(t, entClient, user2, question, "SELECT * FROM test ORDER BY id;", submission.StatusSuccess, now.Add(-30*time.Minute))

	resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

	// Create test server with scope directive
	cfg := Config{
		Resolvers:  resolver,
		Directives: DirectiveRoot{Scope: directive.ScopeDirective},
	}
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	gqlClient := client.New(srv)

	t.Run("success - get all submissions", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID            string
							SubmittedCode string
							Status        string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `) {
					edges {
						node {
							id
							submittedCode
							status
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 4)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)

		// Verify ordering (by ID ascending - default order)
		edges := resp.Me.SubmissionsOfQuestion.Edges
		require.Equal(t, strconv.Itoa(submissions[0].ID), edges[0].Node.ID) // First submission (lowest ID)
		require.Equal(t, strconv.Itoa(submissions[1].ID), edges[1].Node.ID) // Second submission
		require.Equal(t, strconv.Itoa(submissions[2].ID), edges[2].Node.ID) // Third submission
		require.Equal(t, strconv.Itoa(submissions[3].ID), edges[3].Node.ID) // Fourth submission (highest ID)
	})

	t.Run("success - filter by status", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID     string
							Status string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `, where: { status: success }) {
					edges {
						node {
							id
							status
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 2)
		require.Equal(t, 2, resp.Me.SubmissionsOfQuestion.TotalCount)

		// Verify only success status submissions are returned
		for _, edge := range resp.Me.SubmissionsOfQuestion.Edges {
			require.Equal(t, "success", edge.Node.Status)
		}
	})

	t.Run("success - pagination with first parameter", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID string
						}
					}
					PageInfo struct {
						HasNextPage     bool
						HasPreviousPage bool
						StartCursor     string
						EndCursor       string
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `, first: 2) {
					edges {
						node {
							id
						}
					}
					pageInfo {
						hasNextPage
						hasPreviousPage
						startCursor
						endCursor
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 2)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)
		require.True(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasNextPage)
		require.False(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasPreviousPage)
		require.NotEmpty(t, resp.Me.SubmissionsOfQuestion.PageInfo.StartCursor)
		require.NotEmpty(t, resp.Me.SubmissionsOfQuestion.PageInfo.EndCursor)
	})

	t.Run("success - empty results for question with no submissions", func(t *testing.T) {
		// Create another question with no submissions (with unique category)
		emptyQuestion, err := entClient.Question.Create().
			SetCategory("empty-query").
			SetDifficulty("easy").
			SetTitle("Empty Query").
			SetDescription("Write another SELECT query").
			SetReferenceAnswer("SELECT * FROM test WHERE 1=0;").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges      []interface{}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(emptyQuestion.ID) + `) {
					edges {
						node {
							id
						}
					}
					totalCount
				}
			}
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 0)
		require.Equal(t, 0, resp.Me.SubmissionsOfQuestion.TotalCount)
	})

	t.Run("success - filter by failed status", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID            string
							Status        string
							SubmittedCode string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `, where: { status: failed }) {
					edges {
						node {
							id
							status
							submittedCode
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 1)
		require.Equal(t, 1, resp.Me.SubmissionsOfQuestion.TotalCount)
		require.Equal(t, "failed", resp.Me.SubmissionsOfQuestion.Edges[0].Node.Status)
		require.Equal(t, "INVALID SQL;", resp.Me.SubmissionsOfQuestion.Edges[0].Node.SubmittedCode)
	})

	t.Run("error - unauthenticated", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []interface{}
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `) {
					edges {
						node {
							id
						}
					}
				}
			}
		}`

		err := gqlClient.Post(query, &resp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "UNAUTHORIZED")
	})

	t.Run("error - insufficient scope", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []interface{}
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `) {
					edges {
						node {
							id
						}
					}
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:write"}, // Wrong scope
				}),
			)
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "UNAUTHORIZED")
	})

	t.Run("error - non-existent question", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []interface{}
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: 99999) {
					edges {
						node {
							id
						}
					}
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		// Should not error - just return empty results since we're filtering by user and question
		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 0)
	})

	t.Run("success - default ordering (by ID ascending)", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `) {
					edges {
						node {
							id
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 4)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)

		// Verify ordering (by ID ascending - default order)
		edges := resp.Me.SubmissionsOfQuestion.Edges
		require.Equal(t, strconv.Itoa(submissions[0].ID), edges[0].Node.ID) // First submission (lowest ID)
		require.Equal(t, strconv.Itoa(submissions[1].ID), edges[1].Node.ID) // Second submission
		require.Equal(t, strconv.Itoa(submissions[2].ID), edges[2].Node.ID) // Third submission
		require.Equal(t, strconv.Itoa(submissions[3].ID), edges[3].Node.ID) // Fourth submission (highest ID)
	})

	t.Run("success - order by submitted_at ascending (oldest first)", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(
					questionID: ` + strconv.Itoa(question.ID) + `, 
					orderBy: { field: SUBMITTED_AT, direction: ASC }
				) {
					edges {
						node {
							id
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 4)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)

		// Verify ordering (oldest first - by submitted_at ASC)
		edges := resp.Me.SubmissionsOfQuestion.Edges
		require.Equal(t, strconv.Itoa(submissions[0].ID), edges[0].Node.ID) // Oldest submission (-3h)
		require.Equal(t, strconv.Itoa(submissions[1].ID), edges[1].Node.ID) // Second oldest (-2h)
		require.Equal(t, strconv.Itoa(submissions[2].ID), edges[2].Node.ID) // Third oldest (-1h)
		require.Equal(t, strconv.Itoa(submissions[3].ID), edges[3].Node.ID) // Newest submission (now)
	})

	t.Run("success - order by submitted_at descending (newest first)", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(
					questionID: ` + strconv.Itoa(question.ID) + `, 
					orderBy: { field: SUBMITTED_AT, direction: DESC }
				) {
					edges {
						node {
							id
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 4)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)

		// Verify ordering (newest first - by submitted_at DESC)
		edges := resp.Me.SubmissionsOfQuestion.Edges
		require.Equal(t, strconv.Itoa(submissions[3].ID), edges[0].Node.ID) // Newest submission (now)
		require.Equal(t, strconv.Itoa(submissions[2].ID), edges[1].Node.ID) // Third oldest (-1h)
		require.Equal(t, strconv.Itoa(submissions[1].ID), edges[2].Node.ID) // Second oldest (-2h)
		require.Equal(t, strconv.Itoa(submissions[0].ID), edges[3].Node.ID) // Oldest submission (-3h)
	})

	t.Run("success - order by submitted_at ascending with pagination", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID string
						}
					}
					PageInfo struct {
						HasNextPage     bool
						HasPreviousPage bool
						StartCursor     string
						EndCursor       string
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(
					questionID: ` + strconv.Itoa(question.ID) + `, 
					first: 2,
					orderBy: { field: SUBMITTED_AT, direction: ASC }
				) {
					edges {
						node {
							id
						}
					}
					pageInfo {
						hasNextPage
						hasPreviousPage
						startCursor
						endCursor
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 2)
		require.Equal(t, 4, resp.Me.SubmissionsOfQuestion.TotalCount)
		require.True(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasNextPage)
		require.False(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasPreviousPage)

		// Verify ordering with pagination (first 2, oldest first)
		edges := resp.Me.SubmissionsOfQuestion.Edges
		require.Equal(t, strconv.Itoa(submissions[0].ID), edges[0].Node.ID) // Oldest submission (-3h)
		require.Equal(t, strconv.Itoa(submissions[1].ID), edges[1].Node.ID) // Second oldest (-2h)
	})

	t.Run("success - order by submitted_at descending with filter and pagination", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID     string
							Status string
						}
					}
					PageInfo struct {
						HasNextPage     bool
						HasPreviousPage bool
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(
					questionID: ` + strconv.Itoa(question.ID) + `, 
					where: { status: success },
					first: 1,
					orderBy: { field: SUBMITTED_AT, direction: DESC }
				) {
					edges {
						node {
							id
							status
						}
					}
					pageInfo {
						hasNextPage
						hasPreviousPage
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user1.ID,
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 1)
		require.Equal(t, 2, resp.Me.SubmissionsOfQuestion.TotalCount) // Only 2 success submissions
		require.True(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasNextPage)
		require.False(t, resp.Me.SubmissionsOfQuestion.PageInfo.HasPreviousPage)

		// Verify ordering with filter and pagination (newest success first)
		edge := resp.Me.SubmissionsOfQuestion.Edges[0]
		require.Equal(t, strconv.Itoa(submissions[1].ID), edge.Node.ID) // Newer success submission (-2h)
		require.Equal(t, "success", edge.Node.Status)
	})

	t.Run("success - user isolation (user2 doesn't see user1's submissions)", func(t *testing.T) {
		var resp struct {
			Me struct {
				SubmissionsOfQuestion struct {
					Edges []struct {
						Node struct {
							ID            string
							SubmittedCode string
						}
					}
					TotalCount int
				}
			}
		}

		query := `query {
			me {
				submissionsOfQuestion(questionID: ` + strconv.Itoa(question.ID) + `) {
					edges {
						node {
							id
							submittedCode
						}
					}
					totalCount
				}
			}
		}`

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(
				auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: user2.ID, // Different user
					Scopes: []string{"me:read", "submission:read"},
				}),
			)
		})

		require.NoError(t, err)
		require.Len(t, resp.Me.SubmissionsOfQuestion.Edges, 1) // Only user2's submission
		require.Equal(t, 1, resp.Me.SubmissionsOfQuestion.TotalCount)
		require.Equal(t, "SELECT * FROM test ORDER BY id;", resp.Me.SubmissionsOfQuestion.Edges[0].Node.SubmittedCode)
	})
}
