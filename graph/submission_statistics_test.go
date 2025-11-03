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
	"github.com/database-playground/backend-v2/ent/question"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestUserResolver_SubmissionStatistics(t *testing.T) {
	t.Run("user with no submissions", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create a few questions with different difficulties to ensure totalQuestions > 0
		database := createTestDatabase(t, entClient)
		createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyHard)

		// Execute query
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 3, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 0, resp.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 0, resp.User.SubmissionStatistics.SolvedQuestions)
		require.Empty(t, resp.User.SubmissionStatistics.SolvedQuestionByDifficulty)
	})

	t.Run("user with attempted but no solved questions", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database and questions
		database := createTestDatabase(t, entClient)
		easyQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		mediumQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyHard) // Hard question not attempted

		// Create failed submissions for 2 questions
		createTestSubmission(t, entClient, user, easyQuestion, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, user, mediumQuestion, "SELECT * FROM wrong;", submission.StatusFailed, time.Now())

		// Execute query
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 3, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 2, resp.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 0, resp.User.SubmissionStatistics.SolvedQuestions)
		require.Empty(t, resp.User.SubmissionStatistics.SolvedQuestionByDifficulty)
	})

	t.Run("user with solved questions across all difficulties", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database and questions
		database := createTestDatabase(t, entClient)
		easyQuestion1 := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		easyQuestion2 := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		mediumQuestion1 := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		mediumQuestion2 := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		mediumQuestion3 := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		hardQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyHard)

		// Create successful submissions
		createTestSubmission(t, entClient, user, easyQuestion1, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-5*time.Hour))
		createTestSubmission(t, entClient, user, easyQuestion2, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-4*time.Hour))
		createTestSubmission(t, entClient, user, mediumQuestion1, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-3*time.Hour))
		createTestSubmission(t, entClient, user, mediumQuestion2, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-2*time.Hour))
		createTestSubmission(t, entClient, user, mediumQuestion3, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, user, hardQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Execute query
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 6, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 6, resp.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 6, resp.User.SubmissionStatistics.SolvedQuestions)
		require.Len(t, resp.User.SubmissionStatistics.SolvedQuestionByDifficulty, 3)

		// Verify difficulty breakdown
		difficultyMap := make(map[string]int)
		for _, item := range resp.User.SubmissionStatistics.SolvedQuestionByDifficulty {
			difficultyMap[item.Difficulty] = item.SolvedQuestions
		}
		require.Equal(t, 2, difficultyMap["easy"])
		require.Equal(t, 3, difficultyMap["medium"])
		require.Equal(t, 1, difficultyMap["hard"])
	})

	t.Run("user with mixed successful and failed submissions", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database and questions
		database := createTestDatabase(t, entClient)
		easyQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		mediumQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)
		hardQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyHard)
		unsolvedQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)

		// Create mixed submissions
		// Easy question: solved after initial failure
		createTestSubmission(t, entClient, user, easyQuestion, "SELECT * FROM wrong;", submission.StatusFailed, time.Now().Add(-2*time.Hour))
		createTestSubmission(t, entClient, user, easyQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))

		// Medium question: solved
		createTestSubmission(t, entClient, user, mediumQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Hard question: failed attempts only
		createTestSubmission(t, entClient, user, hardQuestion, "SELECT * FROM invalid;", submission.StatusFailed, time.Now().Add(-30*time.Minute))

		// Unsolved question: failed attempts only
		createTestSubmission(t, entClient, user, unsolvedQuestion, "SELECT * FROM bad;", submission.StatusFailed, time.Now().Add(-15*time.Minute))

		// Execute query
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 4, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 4, resp.User.SubmissionStatistics.AttemptedQuestions) // All 4 questions attempted
		require.Equal(t, 2, resp.User.SubmissionStatistics.SolvedQuestions)    // Only 2 questions solved

		// Verify difficulty breakdown (only solved questions should appear)
		require.Len(t, resp.User.SubmissionStatistics.SolvedQuestionByDifficulty, 2)
		difficultyMap := make(map[string]int)
		for _, item := range resp.User.SubmissionStatistics.SolvedQuestionByDifficulty {
			difficultyMap[item.Difficulty] = item.SolvedQuestions
		}
		require.Equal(t, 1, difficultyMap["easy"])   // 1 easy question solved
		require.Equal(t, 1, difficultyMap["medium"]) // 1 medium question solved
		// Hard should not appear in the breakdown since no hard questions were solved
		_, hardExists := difficultyMap["hard"]
		require.False(t, hardExists)
	})

	t.Run("user isolation - statistics don't leak between users", func(t *testing.T) {
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

		// Create test database and questions
		database := createTestDatabase(t, entClient)
		easyQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		mediumQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)

		// Create submissions for user1 only
		createTestSubmission(t, entClient, user1, easyQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, user1, mediumQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Execute query for user1
		var resp1 struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query1 := `query { 
			user(id: ` + strconv.Itoa(user1.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query1, &resp1, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 2, resp1.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 2, resp1.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 2, resp1.User.SubmissionStatistics.SolvedQuestions)

		// Execute query for user2 (should have no submissions)
		var resp2 struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query2 := `query { 
			user(id: ` + strconv.Itoa(user2.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query2, &resp2, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		require.Equal(t, 2, resp2.User.SubmissionStatistics.TotalQuestions)     // Same total questions in system
		require.Equal(t, 0, resp2.User.SubmissionStatistics.AttemptedQuestions) // But user2 has no attempts
		require.Equal(t, 0, resp2.User.SubmissionStatistics.SolvedQuestions)    // And no solved questions
		require.Empty(t, resp2.User.SubmissionStatistics.SolvedQuestionByDifficulty)
	})

	t.Run("direct resolver method call", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test group and user
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database and questions
		database := createTestDatabase(t, entClient)
		easyQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)
		mediumQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyMedium)

		// Create submissions
		createTestSubmission(t, entClient, user, easyQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now().Add(-1*time.Hour))
		createTestSubmission(t, entClient, user, mediumQuestion, "SELECT * FROM wrong;", submission.StatusFailed, time.Now())

		// Test the resolver method directly
		userResolver := &userResolver{resolver}
		stats, err := userResolver.SubmissionStatistics(context.Background(), user)
		require.NoError(t, err)

		require.Equal(t, 2, stats.TotalQuestions)
		require.Equal(t, 2, stats.AttemptedQuestions)
		require.Equal(t, 1, stats.SolvedQuestions)
		require.Len(t, stats.SolvedQuestionByDifficulty, 1)
		require.Equal(t, question.DifficultyEasy, stats.SolvedQuestionByDifficulty[0].Difficulty)
		require.Equal(t, 1, stats.SolvedQuestionByDifficulty[0].SolvedQuestions)
	})

	t.Run("error handling - missing attempted questions count", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)

		// Create test group and user
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Test that the resolver handles missing attempted questions count gracefully
		// This tests the error path in the resolver where the attempted questions query fails
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})
		userResolver := &userResolver{resolver}

		// Close the client to simulate a database error
		entClient.Close()

		_, err = userResolver.SubmissionStatistics(context.Background(), user)
		require.Error(t, err)
		require.Contains(t, err.Error(), "retrieving total questions")
	})

	t.Run("filters by visible_scope - user without scope sees only public questions", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database
		database := createTestDatabase(t, entClient)

		// Create public question (no visible_scope)
		publicQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)

		// Create restricted question (with visible_scope)
		restrictedQuestion, err := entClient.Question.Create().
			SetCategory("premium-query").
			SetDifficulty(question.DifficultyEasy).
			SetTitle("Premium Question").
			SetDescription("Premium question").
			SetReferenceAnswer("SELECT * FROM test;").
			SetVisibleScope("premium:read").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		// Create submissions for both questions
		createTestSubmission(t, entClient, user, publicQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())
		createTestSubmission(t, entClient, user, restrictedQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Query with user without premium:read scope
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"}, // No premium:read
			}))
		})

		require.NoError(t, err)
		// Should only count public question
		require.Equal(t, 1, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 1, resp.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 1, resp.User.SubmissionStatistics.SolvedQuestions)
	})

	t.Run("filters by visible_scope - user with scope sees all questions", func(t *testing.T) {
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

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database
		database := createTestDatabase(t, entClient)

		// Create public question (no visible_scope)
		publicQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)

		// Create restricted question (with visible_scope)
		restrictedQuestion, err := entClient.Question.Create().
			SetCategory("premium-query").
			SetDifficulty(question.DifficultyEasy).
			SetTitle("Premium Question").
			SetDescription("Premium question").
			SetReferenceAnswer("SELECT * FROM test;").
			SetVisibleScope("premium:read").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		// Create submissions for both questions
		createTestSubmission(t, entClient, user, publicQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())
		createTestSubmission(t, entClient, user, restrictedQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Query with user with premium:read scope
		var resp struct {
			User struct {
				SubmissionStatistics struct {
					TotalQuestions             int `json:"totalQuestions"`
					AttemptedQuestions         int `json:"attemptedQuestions"`
					SolvedQuestions            int `json:"solvedQuestions"`
					SolvedQuestionByDifficulty []struct {
						Difficulty      string `json:"difficulty"`
						SolvedQuestions int    `json:"solvedQuestions"`
					} `json:"solvedQuestionByDifficulty"`
				} `json:"submissionStatistics"`
			} `json:"user"`
		}

		query := `query { 
			user(id: ` + strconv.Itoa(user.ID) + `) { 
				submissionStatistics {
					totalQuestions
					attemptedQuestions
					solvedQuestions
					solvedQuestionByDifficulty {
						difficulty
						solvedQuestions
					}
				}
			} 
		}`

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read", "premium:read"}, // Has premium:read
			}))
		})

		require.NoError(t, err)
		// Should count both questions
		require.Equal(t, 2, resp.User.SubmissionStatistics.TotalQuestions)
		require.Equal(t, 2, resp.User.SubmissionStatistics.AttemptedQuestions)
		require.Equal(t, 2, resp.User.SubmissionStatistics.SolvedQuestions)
	})

	t.Run("filters by visible_scope - user with wildcard scope sees all questions", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test group and user
		group, err := createTestGroup(t, entClient)
		require.NoError(t, err)

		user, err := entClient.User.Create().
			SetName("testuser").
			SetEmail("test@example.com").
			SetGroup(group).
			Save(context.Background())
		require.NoError(t, err)

		// Create test database
		database := createTestDatabase(t, entClient)

		// Create public question (no visible_scope)
		publicQuestion := createTestQuestionWithDifficulty(t, entClient, database, question.DifficultyEasy)

		// Create restricted question (with visible_scope)
		restrictedQuestion, err := entClient.Question.Create().
			SetCategory("premium-query").
			SetDifficulty(question.DifficultyEasy).
			SetTitle("Premium Question").
			SetDescription("Premium question").
			SetReferenceAnswer("SELECT * FROM test;").
			SetVisibleScope("premium:read").
			SetDatabase(database).
			Save(context.Background())
		require.NoError(t, err)

		// Create submissions for both questions
		createTestSubmission(t, entClient, user, publicQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())
		createTestSubmission(t, entClient, user, restrictedQuestion, "SELECT * FROM test;", submission.StatusSuccess, time.Now())

		// Test the resolver method directly with wildcard scope
		userResolver := &userResolver{resolver}
		stats, err := userResolver.SubmissionStatistics(auth.WithUser(context.Background(), auth.TokenInfo{
			UserID: user.ID,
			Scopes: []string{"*"},
		}), user)

		require.NoError(t, err)
		// Should count both questions
		require.Equal(t, 2, stats.TotalQuestions)
		require.Equal(t, 2, stats.AttemptedQuestions)
		require.Equal(t, 2, stats.SolvedQuestions)
	})
}

// Helper function to create a question with specific difficulty
func createTestQuestionWithDifficulty(t *testing.T, entClient *ent.Client, database *ent.Database, difficulty question.Difficulty) *ent.Question {
	t.Helper()
	question, err := entClient.Question.Create().
		SetCategory("test-query-" + string(difficulty) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetDifficulty(difficulty).
		SetTitle("Test Query " + string(difficulty)).
		SetDescription("Write a SELECT query for " + string(difficulty) + " difficulty").
		SetReferenceAnswer("SELECT * FROM test;").
		SetDatabase(database).
		Save(context.Background())
	require.NoError(t, err)
	return question
}
