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
	entQuestion "github.com/database-playground/backend-v2/ent/question"
	entSubmission "github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestRankingData creates test users, points, and submissions for ranking tests
func setupTestRankingData(t *testing.T, entClient *ent.Client) ([]*ent.User, *ent.Database, []*ent.Question) {
	t.Helper()

	ctx := context.Background()

	// Create test group
	group, err := createTestGroup(t, entClient)
	require.NoError(t, err)

	// Create test database
	database, err := entClient.Database.Create().
		SetSlug("test_db").
		SetSchema(`{"tables": []}`).
		SetRelationFigure("test_figure").
		Save(ctx)
	require.NoError(t, err)

	// Create test questions
	questions := make([]*ent.Question, 3)
	for i := 0; i < 3; i++ {
		q, err := entClient.Question.Create().
			SetCategory("test").
			SetTitle("Question " + strconv.Itoa(i+1)).
			SetDescription("Test question").
			SetReferenceAnswer("SELECT 1").
			SetDifficulty(entQuestion.DifficultyEasy).
			SetDatabase(database).
			Save(ctx)
		require.NoError(t, err)
		questions[i] = q
	}

	// Create test users
	users := make([]*ent.User, 5)
	for i := 0; i < 5; i++ {
		user, err := entClient.User.Create().
			SetName("User " + strconv.Itoa(i+1)).
			SetEmail("user" + strconv.Itoa(i+1) + "@example.com").
			SetGroup(group).
			Save(ctx)
		require.NoError(t, err)
		users[i] = user
	}

	return users, database, questions
}

func TestQueryResolver_Ranking_ByPoints_Daily(t *testing.T) {
	t.Run("descending order", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for users (today)
		// User 0: 150 points, User 1: 200 points, User 2: 100 points, User 3: 50 points, User 4: 0 points
		pointsData := []struct {
			userIdx int
			points  int
		}{
			{0, 150},
			{1, 200},
			{2, 100},
			{3, 50},
		}

		for _, data := range pointsData {
			_, err := entClient.Point.Create().
				SetUser(users[data.userIdx]).
				SetPoints(data.points).
				SetGrantedAt(today.Add(time.Hour)).
				SetDescription("Daily points").
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						id
						name
					}
				}
				totalCount
				pageInfo {
					hasNextPage
					hasPreviousPage
				}
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						ID   string
						Name string
					}
				}
				TotalCount int
				PageInfo   struct {
					HasNextPage     bool
					HasPreviousPage bool
				}
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 4, resp.Ranking.TotalCount)
		assert.Equal(t, 4, len(resp.Ranking.Edges))
		assert.False(t, resp.Ranking.PageInfo.HasNextPage)
		assert.False(t, resp.Ranking.PageInfo.HasPreviousPage)

		// Check order: User 1 (200), User 0 (150), User 2 (100), User 3 (50)
		assert.Equal(t, "User 2", resp.Ranking.Edges[0].Node.Name)
		assert.Equal(t, "User 1", resp.Ranking.Edges[1].Node.Name)
		assert.Equal(t, "User 3", resp.Ranking.Edges[2].Node.Name)
		assert.Equal(t, "User 4", resp.Ranking.Edges[3].Node.Name)
	})

	t.Run("ascending order", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for users (today)
		pointsData := []struct {
			userIdx int
			points  int
		}{
			{0, 150},
			{1, 200},
			{2, 100},
		}

		for _, data := range pointsData {
			_, err := entClient.Point.Create().
				SetUser(users[data.userIdx]).
				SetPoints(data.points).
				SetGrantedAt(today.Add(time.Hour)).
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: ASC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 3, resp.Ranking.TotalCount)

		// Check order: User 2 (100), User 0 (150), User 1 (200)
		assert.Equal(t, "User 3", resp.Ranking.Edges[0].Node.Name)
		assert.Equal(t, "User 1", resp.Ranking.Edges[1].Node.Name)
		assert.Equal(t, "User 2", resp.Ranking.Edges[2].Node.Name)
	})

	t.Run("filters out yesterday's points", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		yesterday := today.Add(-24 * time.Hour)

		// Create points for yesterday (should not be included)
		_, err := entClient.Point.Create().
			SetUser(users[0]).
			SetPoints(500).
			SetGrantedAt(yesterday.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Create points for today
		_, err = entClient.Point.Create().
			SetUser(users[1]).
			SetPoints(100).
			SetGrantedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response - only user 1 should appear
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Ranking.TotalCount)
		assert.Equal(t, "User 2", resp.Ranking.Edges[0].Node.Name)
	})
}

func TestQueryResolver_Ranking_ByPoints_Weekly(t *testing.T) {
	t.Run("includes entire week", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()

		// Calculate start of week (Monday)
		weekday := now.Weekday()
		daysToMonday := int(weekday - time.Monday)
		if daysToMonday < 0 {
			daysToMonday += 7
		}
		startOfWeek := now.AddDate(0, 0, -daysToMonday)
		startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())

		// Create points throughout the week
		_, err := entClient.Point.Create().
			SetUser(users[0]).
			SetPoints(100).
			SetGrantedAt(startOfWeek.Add(time.Hour)). // Monday
			Save(ctx)
		require.NoError(t, err)

		_, err = entClient.Point.Create().
			SetUser(users[0]).
			SetPoints(50).
			SetGrantedAt(startOfWeek.Add(3 * 24 * time.Hour)). // Thursday
			Save(ctx)
		require.NoError(t, err)

		_, err = entClient.Point.Create().
			SetUser(users[1]).
			SetPoints(200).
			SetGrantedAt(startOfWeek.Add(5 * 24 * time.Hour)). // Saturday
			Save(ctx)
		require.NoError(t, err)

		// Create points from last week (should not be included)
		_, err = entClient.Point.Create().
			SetUser(users[2]).
			SetPoints(1000).
			SetGrantedAt(startOfWeek.Add(-7 * 24 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: WEEKLY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Ranking.TotalCount)

		// User 1 (200), User 0 (150 = 100 + 50)
		assert.Equal(t, "User 2", resp.Ranking.Edges[0].Node.Name)
		assert.Equal(t, "User 1", resp.Ranking.Edges[1].Node.Name)
	})
}

func TestQueryResolver_Ranking_ByCompletedQuestions(t *testing.T) {
	t.Run("counts distinct successful submissions", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, questions := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// User 0: 2 successful submissions (questions 0, 1)
		_, err := entClient.Submission.Create().
			SetUser(users[0]).
			SetQuestion(questions[0]).
			SetSubmittedCode("SELECT 1").
			SetStatus(entSubmission.StatusSuccess).
			SetSubmittedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		_, err = entClient.Submission.Create().
			SetUser(users[0]).
			SetQuestion(questions[1]).
			SetSubmittedCode("SELECT 1").
			SetStatus(entSubmission.StatusSuccess).
			SetSubmittedAt(today.Add(2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// User 1: 1 successful submission (question 0)
		_, err = entClient.Submission.Create().
			SetUser(users[1]).
			SetQuestion(questions[0]).
			SetSubmittedCode("SELECT 1").
			SetStatus(entSubmission.StatusSuccess).
			SetSubmittedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// User 1: 1 failed submission (should not count)
		_, err = entClient.Submission.Create().
			SetUser(users[1]).
			SetQuestion(questions[1]).
			SetSubmittedCode("SELECT wrong").
			SetStatus(entSubmission.StatusFailed).
			SetSubmittedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// User 2: 3 successful submissions (all questions)
		for _, q := range questions {
			_, err = entClient.Submission.Create().
				SetUser(users[2]).
				SetQuestion(q).
				SetSubmittedCode("SELECT 1").
				SetStatus(entSubmission.StatusSuccess).
				SetSubmittedAt(today.Add(time.Hour)).
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: COMPLETED_QUESTIONS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 3, resp.Ranking.TotalCount)

		// Check order: User 2 (3), User 0 (2), User 1 (1)
		assert.Equal(t, "User 3", resp.Ranking.Edges[0].Node.Name)
		assert.Equal(t, "User 1", resp.Ranking.Edges[1].Node.Name)
		assert.Equal(t, "User 2", resp.Ranking.Edges[2].Node.Name)
	})

	t.Run("does not double count same question", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, questions := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// User 0: multiple successful submissions for the same question (should count as 1)
		for i := 0; i < 3; i++ {
			_, err := entClient.Submission.Create().
				SetUser(users[0]).
				SetQuestion(questions[0]).
				SetSubmittedCode("SELECT " + strconv.Itoa(i)).
				SetStatus(entSubmission.StatusSuccess).
				SetSubmittedAt(today.Add(time.Duration(i) * time.Hour)).
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: COMPLETED_QUESTIONS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response - should count as 1 completed question, not 3
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Ranking.TotalCount)
		assert.Equal(t, "User 1", resp.Ranking.Edges[0].Node.Name)
	})
}

func TestQueryResolver_Ranking_Pagination(t *testing.T) {
	t.Run("respects first parameter", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for all users
		for i, user := range users {
			_, err := entClient.Point.Create().
				SetUser(user).
				SetPoints((5 - i) * 100). // Decreasing points
				SetGrantedAt(today.Add(time.Hour)).
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query with first: 3
		query := `query {
			ranking(
				first: 3,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
				pageInfo {
					hasNextPage
					hasPreviousPage
				}
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
				PageInfo   struct {
					HasNextPage     bool
					HasPreviousPage bool
				}
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 5, resp.Ranking.TotalCount)
		assert.Equal(t, 3, len(resp.Ranking.Edges))
		assert.True(t, resp.Ranking.PageInfo.HasNextPage)
		assert.False(t, resp.Ranking.PageInfo.HasPreviousPage)
	})

	t.Run("cursor pagination works", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for all users
		for i, user := range users {
			_, err := entClient.Point.Create().
				SetUser(user).
				SetPoints((5 - i) * 100).
				SetGrantedAt(today.Add(time.Hour)).
				Save(ctx)
			require.NoError(t, err)
		}

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// First query - get first 2
		query1 := `query {
			ranking(
				first: 2,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						id
						name
					}
					cursor
				}
				pageInfo {
					endCursor
					hasNextPage
				}
			}
		}`

		var resp1 struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						ID   string
						Name string
					}
					Cursor string
				}
				PageInfo struct {
					EndCursor   string
					HasNextPage bool
				}
			}
		}

		err := gqlClient.Post(query1, &resp1, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		require.NoError(t, err)
		assert.Equal(t, 2, len(resp1.Ranking.Edges))
		assert.True(t, resp1.Ranking.PageInfo.HasNextPage)

		// Get the cursor from the last edge
		cursor := resp1.Ranking.PageInfo.EndCursor

		// Second query - get next 2 using cursor
		query2 := `query($cursor: Cursor!) {
			ranking(
				first: 2,
				after: $cursor,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				pageInfo {
					hasNextPage
					hasPreviousPage
				}
			}
		}`

		var resp2 struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				PageInfo struct {
					HasNextPage     bool
					HasPreviousPage bool
				}
			}
		}

		err = gqlClient.Post(query2, &resp2,
			client.Var("cursor", cursor),
			func(bd *client.Request) {
				bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
					UserID: 1,
					Scopes: []string{"user:read"},
				}))
			})

		require.NoError(t, err)
		assert.Equal(t, 2, len(resp2.Ranking.Edges))
		assert.True(t, resp2.Ranking.PageInfo.HasPreviousPage)
		assert.True(t, resp2.Ranking.PageInfo.HasNextPage)

		// Verify we got different users
		assert.NotEqual(t, resp1.Ranking.Edges[0].Node.Name, resp2.Ranking.Edges[0].Node.Name)
	})
}

func TestQueryResolver_Ranking_EdgeCases(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Don't create any data

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
				pageInfo {
					hasNextPage
					hasPreviousPage
				}
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
				PageInfo   struct {
					HasNextPage     bool
					HasPreviousPage bool
				}
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Ranking.TotalCount)
		assert.Equal(t, 0, len(resp.Ranking.Edges))
		assert.False(t, resp.Ranking.PageInfo.HasNextPage)
		assert.False(t, resp.Ranking.PageInfo.HasPreviousPage)
	})

	t.Run("single user", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for only one user
		_, err := entClient.Point.Create().
			SetUser(users[0]).
			SetPoints(100).
			SetGrantedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err = gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Ranking.TotalCount)
		assert.Equal(t, 1, len(resp.Ranking.Edges))
		assert.Equal(t, "User 1", resp.Ranking.Edges[0].Node.Name)
	})
}

func TestQueryResolver_Ranking_Authorization(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query without auth
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
			}
		}

		err := gqlClient.Post(query, &resp)

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeUnauthorized)
	})

	t.Run("insufficient scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query with wrong scope
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:write"}, // wrong scope
			}))
		})

		// Verify error
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.CodeForbidden)
	})

	t.Run("with correct scope", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		resolver := NewTestResolver(t, entClient, &mockAuthStorage{})

		// Create test server
		cfg := Config{
			Resolvers:  resolver,
			Directives: DirectiveRoot{Scope: directive.ScopeDirective},
		}
		srv := handler.New(NewExecutableSchema(cfg))
		srv.AddTransport(transport.POST{})
		gqlClient := client.New(srv)

		// Execute query with correct scope
		query := `query {
			ranking(
				first: 10,
				filter: { by: POINTS, order: DESC, period: DAILY }
			) {
				edges {
					node {
						name
					}
				}
				totalCount
			}
		}`

		var resp struct {
			Ranking struct {
				Edges []struct {
					Node struct {
						Name string
					}
				}
				TotalCount int
			}
		}

		err := gqlClient.Post(query, &resp, func(bd *client.Request) {
			bd.HTTP = bd.HTTP.WithContext(auth.WithUser(bd.HTTP.Context(), auth.TokenInfo{
				UserID: 1,
				Scopes: []string{"user:read"},
			}))
		})

		// Verify no error
		require.NoError(t, err)
	})
}
