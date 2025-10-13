package ranking

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/ent"
	entQuestion "github.com/database-playground/backend-v2/ent/question"
	entSubmission "github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/graph/model"
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
	group, err := entClient.Group.Create().
		SetName("Test Group").
		Save(ctx)
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
	for i := range questions {
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
	for i := range 5 {
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

func TestService_GetRanking_ByPoints_Daily(t *testing.T) {
	t.Run("descending order", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

		users, _, _ := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create points for users (today)
		// users[0] (User 1): 150 points, users[1] (User 2): 200 points, users[2] (User 3): 100 points, users[3] (User 4): 50 points
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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 4, result.TotalCount)
		assert.Equal(t, 4, len(result.Edges))
		assert.False(t, result.PageInfo.HasNextPage)
		assert.False(t, result.PageInfo.HasPreviousPage)

		// Check order: User 2 (200), User 1 (150), User 3 (100), User 4 (50)
		assert.Equal(t, "User 2", result.Edges[0].Node.Name)
		assert.Equal(t, "User 1", result.Edges[1].Node.Name)
		assert.Equal(t, "User 3", result.Edges[2].Node.Name)
		assert.Equal(t, "User 4", result.Edges[3].Node.Name)
	})

	t.Run("ascending order", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderAsc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)

		// Check order: User 3 (100), User 1 (150), User 2 (200)
		assert.Equal(t, "User 3", result.Edges[0].Node.Name)
		assert.Equal(t, "User 1", result.Edges[1].Node.Name)
		assert.Equal(t, "User 2", result.Edges[2].Node.Name)
	})

	t.Run("filters out yesterday's points", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response - only user 1 should appear
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "User 2", result.Edges[0].Node.Name)
	})
}

func TestService_GetRanking_ByPoints_Weekly(t *testing.T) {
	t.Run("includes entire week", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodWeekly,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)

		// User 2 (200), User 1 (150 = 100 + 50)
		assert.Equal(t, "User 2", result.Edges[0].Node.Name)
		assert.Equal(t, "User 1", result.Edges[1].Node.Name)
	})
}

func TestService_GetRanking_ByCompletedQuestions(t *testing.T) {
	t.Run("counts distinct successful submissions", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

		users, _, questions := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// users[0] (User 1): 2 successful submissions (questions 0, 1)
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

		// users[1] (User 2): 1 successful submission (question 0)
		_, err = entClient.Submission.Create().
			SetUser(users[1]).
			SetQuestion(questions[0]).
			SetSubmittedCode("SELECT 1").
			SetStatus(entSubmission.StatusSuccess).
			SetSubmittedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// users[1] (User 2): 1 failed submission (should not count)
		_, err = entClient.Submission.Create().
			SetUser(users[1]).
			SetQuestion(questions[1]).
			SetSubmittedCode("SELECT wrong").
			SetStatus(entSubmission.StatusFailed).
			SetSubmittedAt(today.Add(time.Hour)).
			Save(ctx)
		require.NoError(t, err)

		// users[2] (User 3): 3 successful submissions (all questions)
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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByCompletedQuestions,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)

		// Check order: User 3 (3), User 1 (2), User 2 (1)
		assert.Equal(t, "User 3", result.Edges[0].Node.Name)
		assert.Equal(t, "User 1", result.Edges[1].Node.Name)
		assert.Equal(t, "User 2", result.Edges[2].Node.Name)
	})

	t.Run("does not double count same question", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

		users, _, questions := setupTestRankingData(t, entClient)

		ctx := context.Background()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// users[0] (User 1): multiple successful submissions for the same question (should count as 1)
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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByCompletedQuestions,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response - should count as 1 completed question, not 3
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "User 1", result.Edges[0].Node.Name)
	})
}

func TestService_GetRanking_Pagination(t *testing.T) {
	t.Run("respects first parameter", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 3
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 5, result.TotalCount)
		assert.Equal(t, 3, len(result.Edges))
		assert.True(t, result.PageInfo.HasNextPage)
		assert.False(t, result.PageInfo.HasPreviousPage)
	})

	t.Run("cursor pagination works", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// First query - get first 2
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 2
		result1, err := service.GetRanking(ctx, &first, nil, filter)

		require.NoError(t, err)
		assert.Equal(t, 2, len(result1.Edges))
		assert.True(t, result1.PageInfo.HasNextPage)

		// Get the cursor from the last edge
		cursor := result1.PageInfo.EndCursor

		// Second query - get next 2 using cursor
		result2, err := service.GetRanking(ctx, &first, cursor, filter)

		require.NoError(t, err)
		assert.Equal(t, 2, len(result2.Edges))
		assert.True(t, result2.PageInfo.HasPreviousPage)
		assert.True(t, result2.PageInfo.HasNextPage)

		// Verify we got different users
		assert.NotEqual(t, result1.Edges[0].Node.Name, result2.Edges[0].Node.Name)
	})
}

func TestService_GetRanking_EdgeCases(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

		ctx := context.Background()

		// Get ranking without any data
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 0, result.TotalCount)
		assert.Equal(t, 0, len(result.Edges))
		assert.False(t, result.PageInfo.HasNextPage)
		assert.False(t, result.PageInfo.HasPreviousPage)
	})

	t.Run("single user", func(t *testing.T) {
		entClient := testhelper.NewEntSqliteClient(t)
		service := NewService(entClient)

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

		// Get ranking
		filter := model.RankingFilter{
			By:     model.RankingByPoints,
			Order:  model.RankingOrderDesc,
			Period: model.RankingPeriodDaily,
		}
		first := 10
		result, err := service.GetRanking(ctx, &first, nil, filter)

		// Verify response
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, 1, len(result.Edges))
		assert.Equal(t, "User 1", result.Edges[0].Node.Name)
	})
}
