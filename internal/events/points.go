package events

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/event"
	"github.com/database-playground/backend-v2/ent/point"
	"github.com/database-playground/backend-v2/ent/question"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/posthog/posthog-go"
)

// startOfDay returns the start of the given day (midnight).
func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// startOfToday returns the start of today (midnight).
func startOfToday() time.Time {
	return startOfDay(time.Now())
}

const (
	PointDescriptionDailyLogin    = "daily login"
	PointDescriptionWeeklyLogin   = "weekly login"
	PointDescriptionFirstAttempt  = "first attempt on question %d"
	PointDescriptionDailyAttempt  = "daily attempt"
	PointDescriptionCorrectAnswer = "correct answer on question %d"
	PointDescriptionFirstPlace    = "first place on question %d"
)

const (
	PointValueDailyLogin    = 20
	PointValueWeeklyLogin   = 50
	PointValueFirstAttempt  = 30
	PointValueDailyAttempt  = 30
	PointValueCorrectAnswer = 60
	PointValueFirstPlace    = 80
)

// PointsGranter determines if the criteria is met to grant points to a user.
type PointsGranter struct {
	entClient     *ent.Client
	posthogClient posthog.Client
}

// NewPointsGranter creates a new PointsGranter.
func NewPointsGranter(entClient *ent.Client, posthogClient posthog.Client) *PointsGranter {
	return &PointsGranter{
		entClient:     entClient,
		posthogClient: posthogClient,
	}
}

// HandleEvent handles the event creation.
func (d *PointsGranter) HandleEvent(ctx context.Context, event *ent.Event) error {
	switch event.Type {
	case string(EventTypeLogin):
		ok, err := d.GrantDailyLoginPoints(ctx, event.UserID)
		if ok {
			slog.Info("granted daily login points", "user_id", event.UserID)
		}
		return err
	case string(EventTypeSubmitAnswer):
		return d.handleSubmitAnswerEvent(ctx, event)
	}
	return nil
}

// handleSubmitAnswerEvent handles the submit answer event and grants appropriate points.
func (d *PointsGranter) handleSubmitAnswerEvent(ctx context.Context, event *ent.Event) error {
	// Extract submission_id and question_id from payload
	var submissionID int
	switch v := event.Payload["submission_id"].(type) {
	case float64:
		submissionID = int(v)
	case int:
		submissionID = v
	default:
		return fmt.Errorf("submission_id not found in payload or has invalid type")
	}

	var questionID int
	switch v := event.Payload["question_id"].(type) {
	case float64:
		questionID = int(v)
	case int:
		questionID = v
	default:
		return fmt.Errorf("question_id not found in payload or has invalid type")
	}

	// Get the submission to check its status
	sub, err := d.entClient.Submission.Get(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}

	// Grant first attempt points (regardless of correctness)
	ok, err := d.GrantFirstAttemptPoints(ctx, event.UserID, questionID)
	if err != nil {
		return fmt.Errorf("grant first attempt points: %w", err)
	}
	if ok {
		slog.Info("granted first attempt points", "user_id", event.UserID, "question_id", questionID)
	}

	// Grant daily attempt points
	ok, err = d.GrantDailyAttemptPoints(ctx, event.UserID)
	if err != nil {
		return fmt.Errorf("grant daily attempt points: %w", err)
	}
	if ok {
		slog.Info("granted daily attempt points", "user_id", event.UserID)
	}

	// If the submission is successful, grant correct answer and first place points
	if sub.Status == submission.StatusSuccess {
		// Grant correct answer points
		ok, err = d.GrantCorrectAnswerPoints(ctx, event.UserID, questionID)
		if err != nil {
			return fmt.Errorf("grant correct answer points: %w", err)
		}
		if ok {
			slog.Info("granted correct answer points", "user_id", event.UserID, "question_id", questionID)
		}

		// Grant first place points
		ok, err = d.GrantFirstPlacePoints(ctx, event.UserID, questionID)
		if err != nil {
			return fmt.Errorf("grant first place points: %w", err)
		}
		if ok {
			slog.Info("granted first place points", "user_id", event.UserID, "question_id", questionID)
		}
	}

	return nil
}

// GrantDailyLoginPoints grants the "daily login" points to a user.
func (d *PointsGranter) GrantDailyLoginPoints(ctx context.Context, userID int) (bool, error) {
	today := startOfToday()

	// Check if we have granted the "daily login" points for this user today.
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionDailyLogin)).
		Where(point.GrantedAtGTE(today)).Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has logged in today.
	hasTodayLoginRecord, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeLogin))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(today)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if !hasTodayLoginRecord {
		return false, nil
	}

	// Grant the "daily login" points to the user.
	err = d.grantPoint(ctx, userID, 0, PointDescriptionDailyLogin, PointValueDailyLogin)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantWeeklyLoginPoints grants the "weekly login" points to a user.
func (d *PointsGranter) GrantWeeklyLoginPoints(ctx context.Context, userID int) (bool, error) {
	// Calculate the start of 6 days ago (start of the 7-day period)
	sevenDaysAgo := startOfDay(time.Now().AddDate(0, 0, -6))

	// Check if we have granted the "weekly login" points for this user this week.
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionWeeklyLogin)).
		Where(point.GrantedAtGTE(sevenDaysAgo)).Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has logged in every day for the last 7 days.
	weekLoginRecords, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeLogin))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(sevenDaysAgo)).
		All(ctx)
	if err != nil {
		return false, err
	}

	// Aggregated by day
	distinctLoginDays := make(map[time.Time]int)
	for _, record := range weekLoginRecords {
		distinctLoginDays[startOfDay(record.TriggeredAt)]++
	}

	if len(distinctLoginDays) != 7 {
		return false, nil
	}

	// Grant the "weekly login" points to the user.
	err = d.grantPoint(ctx, userID, 0, PointDescriptionWeeklyLogin, PointValueWeeklyLogin)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantFirstAttemptPoints grants the "first attempt" points to a user.
// This is awarded when a user attempts a question for the first time.
func (d *PointsGranter) GrantFirstAttemptPoints(ctx context.Context, userID int, questionID int) (bool, error) {
	// Check if we have granted the "first attempt" points for this user on this question.
	description := fmt.Sprintf(PointDescriptionFirstAttempt, questionID)
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if this is the user's first submission for this question.
	submissionCount, err := d.entClient.Submission.Query().
		Where(submission.HasUserWith(user.ID(userID))).
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Count(ctx)
	if err != nil {
		return false, err
	}
	if submissionCount != 1 {
		return false, nil
	}

	// Grant the "first attempt" points to the user.
	err = d.grantPoint(ctx, userID, questionID, description, PointValueFirstAttempt)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantDailyAttemptPoints grants the "daily attempt" points to a user.
// This is awarded when a user attempts any question today.
func (d *PointsGranter) GrantDailyAttemptPoints(ctx context.Context, userID int) (bool, error) {
	today := startOfToday()

	// Check if we have granted the "daily attempt" points for this user today.
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionDailyAttempt)).
		Where(point.GrantedAtGTE(today)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has submitted any answer today.
	hasSubmittedToday, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeSubmitAnswer))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(today)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if !hasSubmittedToday {
		return false, nil
	}

	// Grant the "daily attempt" points to the user.
	err = d.grantPoint(ctx, userID, 0, PointDescriptionDailyAttempt, PointValueDailyAttempt)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantCorrectAnswerPoints grants the "correct answer" points to a user.
// This is awarded when a user answers a question correctly for the first time.
func (d *PointsGranter) GrantCorrectAnswerPoints(ctx context.Context, userID int, questionID int) (bool, error) {
	// Check if we have granted the "correct answer" points for this user on this question.
	description := fmt.Sprintf(PointDescriptionCorrectAnswer, questionID)
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has a successful submission for this question.
	hasSuccessfulSubmission, err := d.entClient.Submission.Query().
		Where(submission.HasUserWith(user.ID(userID))).
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Where(submission.StatusEQ(submission.StatusSuccess)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if !hasSuccessfulSubmission {
		return false, nil
	}

	// Grant the "correct answer" points to the user.
	err = d.grantPoint(ctx, userID, questionID, description, PointValueCorrectAnswer)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantFirstPlacePoints grants the "first place" points to a user.
// This is awarded when a user is the first to answer a question correctly.
func (d *PointsGranter) GrantFirstPlacePoints(ctx context.Context, userID int, questionID int) (bool, error) {
	// Check if we have granted the "first place" points for any user on this question.
	description := fmt.Sprintf(PointDescriptionFirstPlace, questionID)
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Get the first successful submission for this question.
	firstSuccessfulSubmission, err := d.entClient.Submission.Query().
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Where(submission.StatusEQ(submission.StatusSuccess)).
		Order(submission.BySubmittedAt()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check if this submission belongs to the current user.
	submitterID, err := firstSuccessfulSubmission.QueryUser().OnlyID(ctx)
	if err != nil {
		return false, err
	}
	if submitterID != userID {
		return false, nil
	}

	// Grant the "first place" points to the user.
	err = d.grantPoint(ctx, userID, questionID, description, PointValueFirstPlace)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (d *PointsGranter) grantPoint(ctx context.Context, userID int, questionID int, description string, points int) error {
	err := d.entClient.Point.Create().
		SetUserID(userID).
		SetDescription(description).
		SetPoints(points).
		Exec(ctx)
	if err != nil {
		if d.posthogClient != nil {
			d.posthogClient.Enqueue(posthog.NewDefaultException(
				time.Now(), strconv.Itoa(userID),
				"failed to grant point", err.Error(),
			))
		}

		return err
	}

	if d.posthogClient != nil {
		properties := posthog.NewProperties().
			Set("description", description).
			Set("points", points)

		if questionID != 0 {
			properties.Set("questionID", strconv.Itoa(questionID))
		}

		slog.Debug("sending event to PostHog", "event_type", EventTypeGrantPoint, "user_id", userID)

		d.posthogClient.Enqueue(posthog.Capture{
			DistinctId: strconv.Itoa(userID),
			Event:      string(EventTypeGrantPoint),
			Timestamp:  time.Now(),
			Properties: properties,
		})
	}

	return nil
}
