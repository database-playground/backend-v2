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
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := tracer.Start(ctx, "HandleEvent",
		trace.WithAttributes(
			attribute.String("event.type", event.Type),
			attribute.Int("user.id", event.UserID),
			attribute.Int("event.id", event.ID),
		))
	defer span.End()

	switch event.Type {
	case string(EventTypeLogin):
		span.AddEvent("points.daily_login")
		ok, err := d.GrantDailyLoginPoints(ctx, event.UserID)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to grant daily login points")
			span.RecordError(err)
			return err
		}
		if ok {
			span.AddEvent("points.daily_login.granted")
			slog.Info("granted daily login points", "user_id", event.UserID)
		} else {
			span.AddEvent("points.daily_login.already_granted")
		}
		span.SetStatus(otelcodes.Ok, "Daily login points handled")
		return nil
	case string(EventTypeSubmitAnswer):
		span.AddEvent("points.submit_answer")
		err := d.handleSubmitAnswerEvent(ctx, event)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to handle submit answer event")
			span.RecordError(err)
			return err
		}
		span.SetStatus(otelcodes.Ok, "Submit answer event handled")
		return nil
	}
	span.AddEvent("points.event_type_not_handled")
	span.SetStatus(otelcodes.Ok, "Event type not handled")
	return nil
}

// handleSubmitAnswerEvent handles the submit answer event and grants appropriate points.
func (d *PointsGranter) handleSubmitAnswerEvent(ctx context.Context, event *ent.Event) error {
	ctx, span := tracer.Start(ctx, "handleSubmitAnswerEvent",
		trace.WithAttributes(
			attribute.Int("user.id", event.UserID),
			attribute.Int("event.id", event.ID),
		))
	defer span.End()

	// Extract submission_id and question_id from payload
	span.AddEvent("payload.extraction")
	var submissionID int
	switch v := event.Payload["submission_id"].(type) {
	case float64:
		submissionID = int(v)
	case int:
		submissionID = v
	default:
		span.SetStatus(otelcodes.Error, "submission_id not found in payload or has invalid type")
		return fmt.Errorf("submission_id not found in payload or has invalid type")
	}

	var questionID int
	switch v := event.Payload["question_id"].(type) {
	case float64:
		questionID = int(v)
	case int:
		questionID = v
	default:
		span.SetStatus(otelcodes.Error, "question_id not found in payload or has invalid type")
		return fmt.Errorf("question_id not found in payload or has invalid type")
	}

	span.SetAttributes(
		attribute.Int("submission.id", submissionID),
		attribute.Int("question.id", questionID),
	)

	// Get the submission to check its status
	span.AddEvent("database.submission.get")
	sub, err := d.entClient.Submission.Get(ctx, submissionID)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get submission")
		span.RecordError(err)
		return fmt.Errorf("get submission: %w", err)
	}

	span.SetAttributes(attribute.String("submission.status", string(sub.Status)))

	// Grant first attempt points (regardless of correctness)
	span.AddEvent("points.first_attempt.checking")
	ok, err := d.GrantFirstAttemptPoints(ctx, event.UserID, questionID)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant first attempt points")
		span.RecordError(err)
		return fmt.Errorf("grant first attempt points: %w", err)
	}
	if ok {
		span.AddEvent("points.first_attempt.granted")
		slog.Info("granted first attempt points", "user_id", event.UserID, "question_id", questionID)
	} else {
		span.AddEvent("points.first_attempt.already_granted")
	}

	// Grant daily attempt points
	span.AddEvent("points.daily_attempt.checking")
	ok, err = d.GrantDailyAttemptPoints(ctx, event.UserID)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant daily attempt points")
		span.RecordError(err)
		return fmt.Errorf("grant daily attempt points: %w", err)
	}
	if ok {
		span.AddEvent("points.daily_attempt.granted")
		slog.Info("granted daily attempt points", "user_id", event.UserID)
	} else {
		span.AddEvent("points.daily_attempt.already_granted")
	}

	// If the submission is successful, grant correct answer and first place points
	if sub.Status == submission.StatusSuccess {
		span.AddEvent("submission.success.detected")

		// Grant correct answer points
		span.AddEvent("points.correct_answer.checking")
		ok, err = d.GrantCorrectAnswerPoints(ctx, event.UserID, questionID)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to grant correct answer points")
			span.RecordError(err)
			return fmt.Errorf("grant correct answer points: %w", err)
		}
		if ok {
			span.AddEvent("points.correct_answer.granted")
			slog.Info("granted correct answer points", "user_id", event.UserID, "question_id", questionID)
		} else {
			span.AddEvent("points.correct_answer.already_granted")
		}

		// Grant first place points
		span.AddEvent("points.first_place.checking")
		ok, err = d.GrantFirstPlacePoints(ctx, event.UserID, questionID)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to grant first place points")
			span.RecordError(err)
			return fmt.Errorf("grant first place points: %w", err)
		}
		if ok {
			span.AddEvent("points.first_place.granted")
			slog.Info("granted first place points", "user_id", event.UserID, "question_id", questionID)
		} else {
			span.AddEvent("points.first_place.already_granted")
		}
	} else {
		span.AddEvent("submission.not_successful")
	}

	span.SetStatus(otelcodes.Ok, "Submit answer event handled successfully")
	return nil
}

// GrantDailyLoginPoints grants the "daily login" points to a user.
func (d *PointsGranter) GrantDailyLoginPoints(ctx context.Context, userID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantDailyLoginPoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	today := startOfToday()

	// Check if we have granted the "daily login" points for this user today.
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionDailyLogin)).
		Where(point.GrantedAtGTE(today)).Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "Daily login points already granted")
		return false, nil
	}

	// Check if the user has logged in today.
	span.AddEvent("database.event.check")
	hasTodayLoginRecord, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeLogin))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(today)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check login event")
		span.RecordError(err)
		return false, err
	}
	if !hasTodayLoginRecord {
		span.SetStatus(otelcodes.Ok, "No login event found today")
		return false, nil
	}

	// Grant the "daily login" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, 0, PointDescriptionDailyLogin, PointValueDailyLogin)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant daily login points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "Daily login points granted successfully")
	return true, nil
}

// GrantWeeklyLoginPoints grants the "weekly login" points to a user.
func (d *PointsGranter) GrantWeeklyLoginPoints(ctx context.Context, userID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantWeeklyLoginPoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	// Calculate the start of 6 days ago (start of the 7-day period)
	sevenDaysAgo := startOfDay(time.Now().AddDate(0, 0, -6))

	// Check if we have granted the "weekly login" points for this user this week.
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionWeeklyLogin)).
		Where(point.GrantedAtGTE(sevenDaysAgo)).Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "Weekly login points already granted")
		return false, nil
	}

	// Check if the user has logged in every day for the last 7 days.
	span.AddEvent("database.event.query")
	weekLoginRecords, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeLogin))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(sevenDaysAgo)).
		All(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to query login events")
		span.RecordError(err)
		return false, err
	}

	// Aggregated by day
	span.AddEvent("login.days.aggregation")
	distinctLoginDays := make(map[time.Time]int)
	for _, record := range weekLoginRecords {
		distinctLoginDays[startOfDay(record.TriggeredAt)]++
	}

	span.SetAttributes(
		attribute.Int("login.days.count", len(distinctLoginDays)),
		attribute.Int("login.events.total", len(weekLoginRecords)),
	)

	if len(distinctLoginDays) != 7 {
		span.SetStatus(otelcodes.Ok, "User has not logged in all 7 days")
		return false, nil
	}

	// Grant the "weekly login" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, 0, PointDescriptionWeeklyLogin, PointValueWeeklyLogin)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant weekly login points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "Weekly login points granted successfully")
	return true, nil
}

// GrantFirstAttemptPoints grants the "first attempt" points to a user.
// This is awarded when a user attempts a question for the first time.
func (d *PointsGranter) GrantFirstAttemptPoints(ctx context.Context, userID int, questionID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantFirstAttemptPoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
			attribute.Int("question.id", questionID),
		))
	defer span.End()

	// Check if we have granted the "first attempt" points for this user on this question.
	description := fmt.Sprintf(PointDescriptionFirstAttempt, questionID)
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "First attempt points already granted")
		return false, nil
	}

	// Check if this is the user's first submission for this question.
	span.AddEvent("database.submission.count")
	submissionCount, err := d.entClient.Submission.Query().
		Where(submission.HasUserWith(user.ID(userID))).
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Count(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to count submissions")
		span.RecordError(err)
		return false, err
	}
	span.SetAttributes(attribute.Int("submission.count", submissionCount))
	if submissionCount != 1 {
		span.SetStatus(otelcodes.Ok, "Not the first submission")
		return false, nil
	}

	// Grant the "first attempt" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, questionID, description, PointValueFirstAttempt)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant first attempt points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "First attempt points granted successfully")
	return true, nil
}

// GrantDailyAttemptPoints grants the "daily attempt" points to a user.
// This is awarded when a user attempts any question today.
func (d *PointsGranter) GrantDailyAttemptPoints(ctx context.Context, userID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantDailyAttemptPoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
		))
	defer span.End()

	today := startOfToday()

	// Check if we have granted the "daily attempt" points for this user today.
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(PointDescriptionDailyAttempt)).
		Where(point.GrantedAtGTE(today)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "Daily attempt points already granted")
		return false, nil
	}

	// Check if the user has submitted any answer today.
	span.AddEvent("database.event.check")
	hasSubmittedToday, err := d.entClient.Event.Query().
		Where(event.Type(string(EventTypeSubmitAnswer))).
		Where(event.UserID(userID)).
		Where(event.TriggeredAtGTE(today)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check submit answer event")
		span.RecordError(err)
		return false, err
	}
	if !hasSubmittedToday {
		span.SetStatus(otelcodes.Ok, "No submit answer event found today")
		return false, nil
	}

	// Grant the "daily attempt" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, 0, PointDescriptionDailyAttempt, PointValueDailyAttempt)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant daily attempt points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "Daily attempt points granted successfully")
	return true, nil
}

// GrantCorrectAnswerPoints grants the "correct answer" points to a user.
// This is awarded when a user answers a question correctly for the first time.
func (d *PointsGranter) GrantCorrectAnswerPoints(ctx context.Context, userID int, questionID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantCorrectAnswerPoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
			attribute.Int("question.id", questionID),
		))
	defer span.End()

	// Check if we have granted the "correct answer" points for this user on this question.
	description := fmt.Sprintf(PointDescriptionCorrectAnswer, questionID)
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.HasUserWith(user.ID(userID))).
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "Correct answer points already granted")
		return false, nil
	}

	// Check if the user has a successful submission for this question.
	span.AddEvent("database.submission.check")
	hasSuccessfulSubmission, err := d.entClient.Submission.Query().
		Where(submission.HasUserWith(user.ID(userID))).
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Where(submission.StatusEQ(submission.StatusSuccess)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check successful submission")
		span.RecordError(err)
		return false, err
	}
	if !hasSuccessfulSubmission {
		span.SetStatus(otelcodes.Ok, "No successful submission found")
		return false, nil
	}

	// Grant the "correct answer" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, questionID, description, PointValueCorrectAnswer)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant correct answer points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "Correct answer points granted successfully")
	return true, nil
}

// GrantFirstPlacePoints grants the "first place" points to a user.
// This is awarded when a user is the first to answer a question correctly.
func (d *PointsGranter) GrantFirstPlacePoints(ctx context.Context, userID int, questionID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "GrantFirstPlacePoints",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
			attribute.Int("question.id", questionID),
		))
	defer span.End()

	// Check if we have granted the "first place" points for any user on this question.
	description := fmt.Sprintf(PointDescriptionFirstPlace, questionID)
	span.AddEvent("database.point.check")
	hasPointsRecord, err := d.entClient.Point.Query().
		Where(point.DescriptionEQ(description)).
		Exist(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to check existing points")
		span.RecordError(err)
		return false, err
	}
	if hasPointsRecord {
		span.SetStatus(otelcodes.Ok, "First place points already granted")
		return false, nil
	}

	// Get the first successful submission for this question.
	span.AddEvent("database.submission.first")
	firstSuccessfulSubmission, err := d.entClient.Submission.Query().
		Where(submission.HasQuestionWith(question.IDEQ(questionID))).
		Where(submission.StatusEQ(submission.StatusSuccess)).
		Order(submission.BySubmittedAt()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetStatus(otelcodes.Ok, "No successful submission found")
			return false, nil
		}
		span.SetStatus(otelcodes.Error, "Failed to get first successful submission")
		span.RecordError(err)
		return false, err
	}

	// Check if this submission belongs to the current user.
	span.AddEvent("submission.owner.check")
	submitterID, err := firstSuccessfulSubmission.QueryUser().OnlyID(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to get submission owner")
		span.RecordError(err)
		return false, err
	}
	span.SetAttributes(attribute.Int("submission.owner.id", submitterID))
	if submitterID != userID {
		span.SetStatus(otelcodes.Ok, "User is not the first to answer correctly")
		return false, nil
	}

	// Grant the "first place" points to the user.
	span.AddEvent("points.granting")
	err = d.grantPoint(ctx, userID, questionID, description, PointValueFirstPlace)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to grant first place points")
		span.RecordError(err)
		return false, err
	}

	span.SetStatus(otelcodes.Ok, "First place points granted successfully")
	return true, nil
}

func (d *PointsGranter) grantPoint(ctx context.Context, userID int, questionID int, description string, points int) error {
	ctx, span := tracer.Start(ctx, "grantPoint",
		trace.WithAttributes(
			attribute.Int("user.id", userID),
			attribute.String("point.description", description),
			attribute.Int("point.value", points),
		))
	defer span.End()

	if questionID != 0 {
		span.SetAttributes(attribute.Int("question.id", questionID))
	}

	span.AddEvent("database.point.create")
	err := d.entClient.Point.Create().
		SetUserID(userID).
		SetDescription(description).
		SetPoints(points).
		Exec(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create point")
		span.RecordError(err)

		if d.posthogClient != nil {
			span.AddEvent("posthog.exception.sending")
			err = d.posthogClient.Enqueue(posthog.NewDefaultException(
				time.Now(), strconv.Itoa(userID),
				"failed to grant point", err.Error(),
			))
			if err != nil {
				span.RecordError(err)
				slog.Error("failed to send event to PostHog", "error", err)
			}
		}

		return err
	}

	span.AddEvent("database.point.created")

	if d.posthogClient != nil {
		span.AddEvent("posthog.capture")
		properties := posthog.NewProperties().
			Set("description", description).
			Set("points", points)

		if questionID != 0 {
			properties.Set("questionID", strconv.Itoa(questionID))
		}

		slog.Debug("sending event to PostHog", "event_type", EventTypeGrantPoint, "user_id", userID)

		err = d.posthogClient.Enqueue(posthog.Capture{
			DistinctId: strconv.Itoa(userID),
			Event:      string(EventTypeGrantPoint),
			Timestamp:  time.Now(),
			Properties: properties,
		})
		if err != nil {
			span.RecordError(err)
			slog.Error("failed to send event to PostHog", "error", err)
		}
	}

	span.SetStatus(otelcodes.Ok, "Point granted successfully")
	return nil
}
