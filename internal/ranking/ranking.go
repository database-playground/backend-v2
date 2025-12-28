package ranking

import (
	"context"
	"fmt"
	"sort"
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/dialect/sql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/point"
	entSubmission "github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/graph/model"
	"github.com/database-playground/backend-v2/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("dbplay.ranking")

// Service handles ranking operations
type Service struct {
	client *ent.Client
}

// NewService creates a new ranking service
func NewService(client *ent.Client) *Service {
	return &Service{
		client: client,
	}
}

// GetRanking retrieves the ranking based on the provided filter and pagination parameters
func (s *Service) GetRanking(ctx context.Context, first *int, after *entgql.Cursor[int], filter model.RankingFilter) (*model.RankingConnection, error) {
	ctx, span := tracer.Start(ctx, "GetRanking",
		trace.WithAttributes(
			attribute.String("ranking.by", string(filter.By)),
			attribute.String("ranking.period", string(filter.Period)),
			attribute.String("ranking.order", string(filter.Order)),
		))
	defer span.End()

	// Calculate the time range based on the period
	span.AddEvent("time_range.calculating")
	timeRange := s.getTimeRange(filter.Period)
	span.SetAttributes(attribute.String("ranking.time_range.start", timeRange.Format(time.RFC3339)))

	// Get all users with their scores
	var userScores []models.UserScore
	var err error

	switch filter.By {
	case model.RankingByPoints:
		span.AddEvent("ranking.by_points")
		userScores, err = s.getUserScoresByPoints(ctx, timeRange)
	case model.RankingByCompletedQuestions:
		span.AddEvent("ranking.by_completed_questions")
		userScores, err = s.getUserScoresByCompletedQuestions(ctx, timeRange)
	default:
		span.SetStatus(otelcodes.Error, fmt.Sprintf("Unsupported ranking type: %s", filter.By))
		return nil, fmt.Errorf("unsupported ranking type: %s", filter.By)
	}

	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get user scores")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get user scores: %w", err)
	}

	// Sort based on order
	span.AddEvent("sorting.started")
	s.sortUserScores(userScores, filter.Order)

	// Calculate total count before pagination
	totalCount := len(userScores)
	span.SetAttributes(attribute.Int("ranking.total_count", totalCount))

	// Apply cursor pagination
	startIdx := 0
	if after != nil {
		span.AddEvent("pagination.cursor_applied")
		// Find the position after the cursor
		afterID := after.ID
		for i, us := range userScores {
			if us.UserID == afterID {
				startIdx = i + 1
				break
			}
		}
	}

	// Apply limit
	limit := 10 // default limit
	if first != nil && *first > 0 {
		limit = *first
	}
	span.SetAttributes(
		attribute.Int("ranking.limit", limit),
		attribute.Int("ranking.start_index", startIdx),
	)

	endIdx := min(startIdx+limit, len(userScores))
	paginatedScores := userScores[startIdx:endIdx]

	// Fetch user entities for the paginated results
	span.AddEvent("database.users.fetching")
	userIDs := make([]int, len(paginatedScores))
	for i, us := range paginatedScores {
		userIDs[i] = us.UserID
	}

	users, err := s.client.User.Query().
		Where(user.IDIn(userIDs...)).
		All(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to fetch users")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	// Create a map for quick lookup
	userMap := make(map[int]*ent.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	// Build edges
	span.AddEvent("edges.building")
	edges := make([]*model.RankingEdge, 0, len(paginatedScores))
	for _, us := range paginatedScores {
		if user, ok := userMap[us.UserID]; ok {
			cursor := entgql.Cursor[int]{ID: us.UserID}
			edges = append(edges, &model.RankingEdge{
				Node:   user,
				Score:  us.Score,
				Cursor: cursor,
			})
		}
	}

	// Build page info
	pageInfo := &entgql.PageInfo[int]{
		HasNextPage:     endIdx < len(userScores),
		HasPreviousPage: startIdx > 0,
	}

	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	span.SetAttributes(
		attribute.Int("ranking.edges_count", len(edges)),
		attribute.Bool("ranking.has_next_page", pageInfo.HasNextPage),
		attribute.Bool("ranking.has_previous_page", pageInfo.HasPreviousPage),
	)

	span.SetStatus(otelcodes.Ok, "Ranking retrieved successfully")
	return &model.RankingConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: totalCount,
	}, nil
}

// getTimeRange calculates the start time based on the period
func (s *Service) getTimeRange(period model.RankingPeriod) time.Time {
	now := time.Now()

	switch period {
	case model.RankingPeriodDaily:
		// Start of today
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case model.RankingPeriodWeekly:
		// Start of the week (Monday)
		weekday := now.Weekday()
		daysToMonday := int(weekday - time.Monday)
		if daysToMonday < 0 {
			daysToMonday += 7
		}
		startOfWeek := now.AddDate(0, 0, -daysToMonday)
		return time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())
	default:
		// Default to daily
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
}

// getUserScoresByPoints gets user scores based on total points in the time range
func (s *Service) getUserScoresByPoints(ctx context.Context, since time.Time) ([]models.UserScore, error) {
	ctx, span := tracer.Start(ctx, "getUserScoresByPoints",
		trace.WithAttributes(
			attribute.String("ranking.time_range.start", since.Format(time.RFC3339)),
		))
	defer span.End()

	var results []struct {
		UserID     int `json:"user_points"`
		TotalScore int `json:"total_score"`
	}

	span.AddEvent("database.points.querying")
	err := s.client.Point.Query().
		Where(point.GrantedAtGTE(since)).
		GroupBy("user_points").
		Aggregate(func(sel *sql.Selector) string {
			return sql.As(sql.Sum(point.FieldPoints), "total_score")
		}).
		Scan(ctx, &results)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to query points")
		span.RecordError(err)
		return nil, err
	}

	span.AddEvent("results.processing")
	userScores := make([]models.UserScore, len(results))
	for i, r := range results {
		userScores[i] = models.UserScore{
			UserID: r.UserID,
			Score:  r.TotalScore,
		}
	}

	span.SetAttributes(attribute.Int("ranking.users_count", len(userScores)))
	span.SetStatus(otelcodes.Ok, "User scores by points retrieved successfully")
	return userScores, nil
}

// getUserScoresByCompletedQuestions gets user scores based on completed questions in the time range
func (s *Service) getUserScoresByCompletedQuestions(ctx context.Context, since time.Time) ([]models.UserScore, error) {
	ctx, span := tracer.Start(ctx, "getUserScoresByCompletedQuestions",
		trace.WithAttributes(
			attribute.String("ranking.time_range.start", since.Format(time.RFC3339)),
		))
	defer span.End()

	var results []struct {
		UserID          int `json:"user_submissions"`
		CompletedQuests int `json:"completed_quests"`
	}

	// Count distinct successful submissions per user
	span.AddEvent("database.submissions.querying")
	err := s.client.Submission.Query().
		Where(
			entSubmission.StatusEQ(entSubmission.StatusSuccess),
			entSubmission.SubmittedAtGTE(since),
		).
		GroupBy("user_submissions").
		Aggregate(func(sel *sql.Selector) string {
			// Count distinct questions
			return sql.As(fmt.Sprintf("COUNT(DISTINCT %s)", "question_submissions"), "completed_quests")
		}).
		Scan(ctx, &results)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to query submissions")
		span.RecordError(err)
		return nil, err
	}

	span.AddEvent("results.processing")
	userScores := make([]models.UserScore, len(results))
	for i, r := range results {
		userScores[i] = models.UserScore{
			UserID: r.UserID,
			Score:  r.CompletedQuests,
		}
	}

	span.SetAttributes(attribute.Int("ranking.users_count", len(userScores)))
	span.SetStatus(otelcodes.Ok, "User scores by completed questions retrieved successfully")
	return userScores, nil
}

// sortUserScores sorts user scores in place based on the order
func (s *Service) sortUserScores(scores []models.UserScore, order model.RankingOrder) {
	if order == model.RankingOrderDesc {
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score > scores[j].Score
		})
	} else {
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score < scores[j].Score
		})
	}
}
