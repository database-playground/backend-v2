package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/point"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/ent/user"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/setup"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestData creates a user and returns the client and user ID for testing
func setupTestData(t *testing.T, client *ent.Client) int {
	t.Helper()

	ctx := context.Background()

	// Setup the database with required groups and scope sets
	setupResult, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	// Create a user for testing with the new user group
	user, err := client.User.Create().
		SetName("Test User").
		SetEmail("test@example.com").
		SetGroup(setupResult.NewUserGroup).
		Save(ctx)
	require.NoError(t, err)

	return user.ID
}

// createLoginEvent creates a login event for the user at the specified time
func createLoginEvent(t *testing.T, client *ent.Client, userID int, triggeredAt time.Time) {
	t.Helper()

	ctx := context.Background()

	_, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeLogin)).
		SetTriggeredAt(triggeredAt).
		Save(ctx)
	require.NoError(t, err)
}

// createPointsRecord creates a points record for the user with specified created_at time
func createPointsRecord(t *testing.T, client *ent.Client, userID int, description string, pointsValue int, grantedAt time.Time) {
	t.Helper()

	ctx := context.Background()

	// Create the points record with specified created_at time
	_, err := client.Point.Create().
		SetUserID(userID).
		SetDescription(description).
		SetPoints(pointsValue).
		SetGrantedAt(grantedAt).
		Save(ctx)
	require.NoError(t, err)
}

func TestGrantDailyLoginPoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create a login event from today
	createLoginEvent(t, client, userID, now)

	// Grant daily login points
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueDailyLogin, pointsRecords[0].Points)
}

func TestGrantDailyLoginPoints_AlreadyGrantedToday(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create a login event from today
	createLoginEvent(t, client, userID, now)

	// Create an existing points record from today
	createPointsRecord(t, client, userID, events.PointDescriptionDailyLogin, events.PointValueDailyLogin, now)

	// Attempt to grant daily login points again
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestGrantDailyLoginPoints_NoLoginToday(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	yesterday := time.Now().AddDate(0, 0, -2) // 2 days ago to be sure it's outside the window

	// Create a login event from yesterday (outside the 24 hour window)
	createLoginEvent(t, client, userID, yesterday)

	// Attempt to grant daily login points
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantDailyLoginPoints_OldPointsRecordExists(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()
	twoDaysAgo := now.AddDate(0, 0, -2)

	// Create a login event from today
	createLoginEvent(t, client, userID, now)

	// Create an old points record from 2 days ago
	createPointsRecord(t, client, userID, events.PointDescriptionDailyLogin, events.PointValueDailyLogin, twoDaysAgo)

	// Grant daily login points should succeed since old record is outside 24 hour window
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify two points records exist now
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 2)
}

func TestGrantWeeklyLoginPoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create login events for 7 consecutive days
	for i := 0; i < 7; i++ {
		loginTime := now.AddDate(0, 0, -i)
		createLoginEvent(t, client, userID, loginTime)
	}

	// Grant weekly login points
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueWeeklyLogin, pointsRecords[0].Points)
}

func TestGrantWeeklyLoginPoints_AlreadyGrantedThisWeek(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create login events for 7 consecutive days
	for i := 0; i < 7; i++ {
		loginTime := now.AddDate(0, 0, -i)
		createLoginEvent(t, client, userID, loginTime)
	}

	// Create an existing weekly points record from this week
	createPointsRecord(t, client, userID, events.PointDescriptionWeeklyLogin, events.PointValueWeeklyLogin, now)

	// Attempt to grant weekly login points again
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestGrantWeeklyLoginPoints_InsufficientLoginDays(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create login events for only 5 days (insufficient for weekly points)
	for i := 0; i < 5; i++ {
		loginTime := now.AddDate(0, 0, -i)
		createLoginEvent(t, client, userID, loginTime)
	}

	// Attempt to grant weekly login points
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantWeeklyLoginPoints_NoLoginEvents(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()

	// Don't create any login events

	// Attempt to grant weekly login points
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantWeeklyLoginPoints_MultipleLoginsPerDay(t *testing.T) {
	testCases := []struct {
		name        string
		days        int
		shouldGrant bool
		description string
	}{
		{
			name:        "SufficientDays",
			days:        7,
			shouldGrant: true,
			description: "Should grant points with 7 days of multiple logins per day",
		},
		{
			name:        "InsufficientDays",
			days:        6,
			shouldGrant: false,
			description: "Should not grant points with only 6 days of multiple logins per day",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := testhelper.NewEntSqliteClient(t)
			granter := events.NewPointsGranter(client, nil)
			userID := setupTestData(t, client)

			ctx := context.Background()
			now := time.Now()

			// Create multiple login events for specified number of consecutive days
			for i := 0; i < tc.days; i++ {
				dayStart := now.AddDate(0, 0, -i)
				// Create 3 login events for each day at different times
				for j := 0; j < 3; j++ {
					loginTime := dayStart.Add(time.Duration(j) * time.Hour)
					createLoginEvent(t, client, userID, loginTime)
				}
			}

			// Grant weekly login points
			granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, tc.shouldGrant, granted)

			// Verify points were created or not created based on expectation
			pointsRecords, err := client.Point.Query().
				Where(point.HasUserWith(user.IDEQ(userID))).
				Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
				All(ctx)
			require.NoError(t, err)

			if tc.shouldGrant {
				require.Len(t, pointsRecords, 1)
				require.Equal(t, events.PointValueWeeklyLogin, pointsRecords[0].Points)
			} else {
				require.Len(t, pointsRecords, 0)
			}
		})
	}
}

func TestGrantWeeklyLoginPoints_OldWeeklyPointsRecordExists(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()
	tenDaysAgo := now.AddDate(0, 0, -10)

	// Create login events for 7 consecutive days
	for i := 0; i < 7; i++ {
		loginTime := now.AddDate(0, 0, -i)
		createLoginEvent(t, client, userID, loginTime)
	}

	// Create an old weekly points record from 10 days ago
	createPointsRecord(t, client, userID, events.PointDescriptionWeeklyLogin, events.PointValueWeeklyLogin, tenDaysAgo)

	// Grant weekly login points should succeed since old record is outside 7 day window
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify two points records exist now
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 2)
}

func TestNewPointsGranter(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)

	granter := events.NewPointsGranter(client, nil)
	require.NotNil(t, granter)
}

func TestGrantDailyLoginPoints_NonExistentUser(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)

	ctx := context.Background()
	nonExistentUserID := 99999

	// Attempt to grant points for non-existent user should not fail during the query phase
	// but should return false since there's no login event
	granted, err := granter.GrantDailyLoginPoints(ctx, nonExistentUserID)
	require.NoError(t, err)
	require.False(t, granted)
}

func TestGrantWeeklyLoginPoints_NonExistentUser(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)

	ctx := context.Background()
	nonExistentUserID := 99999

	// Attempt to grant points for non-existent user should not fail during the query phase
	// but should return false since there are no login events
	granted, err := granter.GrantWeeklyLoginPoints(ctx, nonExistentUserID)
	require.NoError(t, err)
	require.False(t, granted)
}

// createQuestion creates a question for testing
func createQuestion(t *testing.T, client *ent.Client, databaseID int) int {
	t.Helper()

	ctx := context.Background()

	q, err := client.Question.Create().
		SetCategory("test-category").
		SetTitle("Test Question").
		SetDescription("Test Description").
		SetReferenceAnswer("SELECT * FROM test;").
		SetDatabaseID(databaseID).
		Save(ctx)
	require.NoError(t, err)

	return q.ID
}

// createDatabase creates a database for testing
func createDatabase(t *testing.T, client *ent.Client) int {
	t.Helper()

	ctx := context.Background()

	db, err := client.Database.Create().
		SetSlug("test-db").
		SetDescription("Test Database").
		SetSchema("CREATE TABLE test (id INT);").
		SetRelationFigure("https://example.com/test-db-relation.png").
		Save(ctx)
	require.NoError(t, err)

	return db.ID
}

// createSubmission creates a submission for testing
func createSubmission(t *testing.T, client *ent.Client, userID int, questionID int, status submission.Status, submittedAt time.Time) int {
	t.Helper()

	ctx := context.Background()

	sub, err := client.Submission.Create().
		SetUserID(userID).
		SetQuestionID(questionID).
		SetSubmittedCode("SELECT * FROM test;").
		SetStatus(status).
		SetSubmittedAt(submittedAt).
		Save(ctx)
	require.NoError(t, err)

	return sub.ID
}

// createSubmitAnswerEvent creates a submit answer event for testing
func createSubmitAnswerEvent(t *testing.T, client *ent.Client, userID int, submissionID int, questionID int, triggeredAt time.Time) {
	t.Helper()

	ctx := context.Background()

	_, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeSubmitAnswer)).
		SetPayload(map[string]any{
			// ent requires float64 for JSON serialization
			"submission_id": float64(submissionID),
			"question_id":   float64(questionID),
		}).
		SetTriggeredAt(triggeredAt).
		Save(ctx)
	require.NoError(t, err)
}

func TestGrantFirstAttemptPoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Create a database and question
	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create the first submission for this user on this question
	createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Grant first attempt points
	granted, err := granter.GrantFirstAttemptPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueFirstAttempt, pointsRecords[0].Points)
}

func TestGrantFirstAttemptPoints_AlreadyGranted(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create a submission
	createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Grant points once
	granted, err := granter.GrantFirstAttemptPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Attempt to grant again
	granted, err = granter.GrantFirstAttemptPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestGrantFirstAttemptPoints_SecondSubmission(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create two submissions
	createSubmission(t, client, userID, questionID, submission.StatusFailed, now)
	createSubmission(t, client, userID, questionID, submission.StatusSuccess, now.Add(time.Minute))

	// Attempt to grant first attempt points (should fail because there are 2 submissions)
	granted, err := granter.GrantFirstAttemptPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantDailyAttemptPoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)
	submissionID := createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Create a submit answer event from today
	createSubmitAnswerEvent(t, client, userID, submissionID, questionID, now)

	// Grant daily attempt points
	granted, err := granter.GrantDailyAttemptPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyAttempt)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueDailyAttempt, pointsRecords[0].Points)
}

func TestGrantDailyAttemptPoints_AlreadyGrantedToday(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)
	submissionID := createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Create a submit answer event from today
	createSubmitAnswerEvent(t, client, userID, submissionID, questionID, now)

	// Create an existing points record from today
	createPointsRecord(t, client, userID, events.PointDescriptionDailyAttempt, events.PointValueDailyAttempt, now)

	// Attempt to grant daily attempt points again
	granted, err := granter.GrantDailyAttemptPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyAttempt)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestGrantDailyAttemptPoints_NoSubmissionToday(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	twoDaysAgo := time.Now().AddDate(0, 0, -2)

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)
	submissionID := createSubmission(t, client, userID, questionID, submission.StatusFailed, twoDaysAgo)

	// Create a submit answer event from 2 days ago
	createSubmitAnswerEvent(t, client, userID, submissionID, questionID, twoDaysAgo)

	// Attempt to grant daily attempt points
	granted, err := granter.GrantDailyAttemptPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyAttempt)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantCorrectAnswerPoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create a successful submission
	createSubmission(t, client, userID, questionID, submission.StatusSuccess, now)

	// Grant correct answer points
	granted, err := granter.GrantCorrectAnswerPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueCorrectAnswer, pointsRecords[0].Points)
}

func TestGrantCorrectAnswerPoints_AlreadyGranted(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create a successful submission
	createSubmission(t, client, userID, questionID, submission.StatusSuccess, now)

	// Grant points once
	granted, err := granter.GrantCorrectAnswerPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Attempt to grant again
	granted, err = granter.GrantCorrectAnswerPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestGrantCorrectAnswerPoints_NoSuccessfulSubmission(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create a failed submission
	createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Attempt to grant correct answer points
	granted, err := granter.GrantCorrectAnswerPoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantFirstPlacePoints_Success(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create the first successful submission
	createSubmission(t, client, userID, questionID, submission.StatusSuccess, now)

	// Grant first place points
	granted, err := granter.GrantFirstPlacePoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueFirstPlace, pointsRecords[0].Points)
}

func TestGrantFirstPlacePoints_NotFirstPlace(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)

	ctx := context.Background()
	now := time.Now()

	// Create two users
	userID1 := setupTestData(t, client)
	setupResult, err := setup.Setup(ctx, client)
	require.NoError(t, err)

	user2, err := client.User.Create().
		SetName("Test User 2").
		SetEmail("test2@example.com").
		SetGroup(setupResult.NewUserGroup).
		Save(ctx)
	require.NoError(t, err)
	userID2 := user2.ID

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// User 1 submits successfully first
	createSubmission(t, client, userID1, questionID, submission.StatusSuccess, now)

	// User 2 submits successfully later
	createSubmission(t, client, userID2, questionID, submission.StatusSuccess, now.Add(time.Minute))

	// Attempt to grant first place points to user 2
	granted, err := granter.GrantFirstPlacePoints(ctx, userID2, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify no points record was created for user 2
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID2))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}

func TestGrantFirstPlacePoints_AlreadyGranted(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create the first successful submission
	createSubmission(t, client, userID, questionID, submission.StatusSuccess, now)

	// Grant first place points once
	granted, err := granter.GrantFirstPlacePoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.True(t, granted)

	// Attempt to grant again
	granted, err = granter.GrantFirstPlacePoints(ctx, userID, questionID)
	require.NoError(t, err)
	require.False(t, granted)

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

func TestHandleSubmitAnswerEvent_SuccessfulSubmission(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create the first successful submission
	submissionID := createSubmission(t, client, userID, questionID, submission.StatusSuccess, now)

	// Create and handle the event
	event, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeSubmitAnswer)).
		SetPayload(map[string]any{
			"submission_id": float64(submissionID),
			"question_id":   float64(questionID),
		}).
		SetTriggeredAt(now).
		Save(ctx)
	require.NoError(t, err)

	err = granter.HandleEvent(ctx, event)
	require.NoError(t, err)

	// Verify all appropriate points were granted
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 4)

	totalPoints := 0
	for _, p := range pointsRecords {
		totalPoints += p.Points
	}
	expectedTotal := events.PointValueFirstAttempt + events.PointValueDailyAttempt + events.PointValueCorrectAnswer + events.PointValueFirstPlace
	require.Equal(t, expectedTotal, totalPoints)
}

func TestHandleSubmitAnswerEvent_FailedSubmission(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Create a failed submission
	submissionID := createSubmission(t, client, userID, questionID, submission.StatusFailed, now)

	// Create and handle the event
	event, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeSubmitAnswer)).
		SetPayload(map[string]any{
			"submission_id": float64(submissionID),
			"question_id":   float64(questionID),
		}).
		SetTriggeredAt(now).
		Save(ctx)
	require.NoError(t, err)

	err = granter.HandleEvent(ctx, event)
	require.NoError(t, err)

	// Verify only first attempt and daily attempt points were granted
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 2)

	totalPoints := 0
	for _, p := range pointsRecords {
		totalPoints += p.Points
	}
	expectedTotal := events.PointValueFirstAttempt + events.PointValueDailyAttempt
	require.Equal(t, expectedTotal, totalPoints)
}

func TestHandleSubmitAnswerEvent_SecondAttemptSuccess(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// First attempt - failed
	submission1ID := createSubmission(t, client, userID, questionID, submission.StatusFailed, now)
	event1, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeSubmitAnswer)).
		SetPayload(map[string]any{
			"submission_id": float64(submission1ID),
			"question_id":   float64(questionID),
		}).
		SetTriggeredAt(now).
		Save(ctx)
	require.NoError(t, err)

	err = granter.HandleEvent(ctx, event1)
	require.NoError(t, err)

	// Second attempt - success
	submission2ID := createSubmission(t, client, userID, questionID, submission.StatusSuccess, now.Add(time.Minute))
	event2, err := client.Event.Create().
		SetUserID(userID).
		SetType(string(events.EventTypeSubmitAnswer)).
		SetPayload(map[string]any{
			"submission_id": float64(submission2ID),
			"question_id":   float64(questionID),
		}).
		SetTriggeredAt(now.Add(time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	err = granter.HandleEvent(ctx, event2)
	require.NoError(t, err)

	// Verify points were granted correctly
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 4)

	totalPoints := 0
	for _, p := range pointsRecords {
		totalPoints += p.Points
	}
	expectedTotal := events.PointValueFirstAttempt + events.PointValueDailyAttempt + events.PointValueCorrectAnswer + events.PointValueFirstPlace
	require.Equal(t, expectedTotal, totalPoints)
}

// TestGrantDailyLoginPoints_MidnightBoundary tests the edge case where events happen around midnight
func TestGrantDailyLoginPoints_MidnightBoundary(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Get today's date at 11:59 PM (just before midnight)
	year, month, day := now.Date()
	yesterdayNight := time.Date(year, month, day-1, 23, 59, 59, 0, now.Location())

	// Create a login event from yesterday at 11:59 PM
	createLoginEvent(t, client, userID, yesterdayNight)

	// Create a points record from yesterday
	createPointsRecord(t, client, userID, events.PointDescriptionDailyLogin, events.PointValueDailyLogin, yesterdayNight)

	// Now create a login event from today at 12:01 AM (just after midnight)
	todayMorning := time.Date(year, month, day, 0, 1, 0, 0, now.Location())
	createLoginEvent(t, client, userID, todayMorning)

	// Grant daily login points should succeed because the old record is from yesterday
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted, "Should grant points because yesterday's 11:59 PM and today's 12:01 AM are different days")

	// Verify two points records exist now (one from yesterday, one from today)
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 2, "Should have two records: one from yesterday and one from today")
}

// TestGrantDailyLoginPoints_SameDayDifferentTimes tests that multiple events on the same day only grant points once
func TestGrantDailyLoginPoints_SameDayDifferentTimes(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Get today's date
	year, month, day := now.Date()

	// Create a login event at 8:00 AM today
	morning := time.Date(year, month, day, 8, 0, 0, 0, now.Location())
	createLoginEvent(t, client, userID, morning)

	// Grant daily login points
	granted, err := granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted)

	// Create another login event at 11:00 PM today (same day)
	evening := time.Date(year, month, day, 23, 0, 0, 0, now.Location())
	createLoginEvent(t, client, userID, evening)

	// Attempt to grant daily login points again
	granted, err = granter.GrantDailyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted, "Should not grant points again on the same day")

	// Verify only one points record exists
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
}

// TestGrantDailyAttemptPoints_MidnightBoundary tests the edge case for daily attempts around midnight
func TestGrantDailyAttemptPoints_MidnightBoundary(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	databaseID := createDatabase(t, client)
	questionID := createQuestion(t, client, databaseID)

	// Get today's date
	year, month, day := now.Date()

	// Create a submission and event from yesterday at 11:59 PM
	yesterdayNight := time.Date(year, month, day-1, 23, 59, 59, 0, now.Location())
	submissionID1 := createSubmission(t, client, userID, questionID, submission.StatusFailed, yesterdayNight)
	createSubmitAnswerEvent(t, client, userID, submissionID1, questionID, yesterdayNight)

	// Create a points record from yesterday
	createPointsRecord(t, client, userID, events.PointDescriptionDailyAttempt, events.PointValueDailyAttempt, yesterdayNight)

	// Create a submission and event from today at 12:01 AM
	todayMorning := time.Date(year, month, day, 0, 1, 0, 0, now.Location())
	submissionID2 := createSubmission(t, client, userID, questionID, submission.StatusFailed, todayMorning)
	createSubmitAnswerEvent(t, client, userID, submissionID2, questionID, todayMorning)

	// Grant daily attempt points should succeed because the old record is from yesterday
	granted, err := granter.GrantDailyAttemptPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted, "Should grant points because yesterday's 11:59 PM and today's 12:01 AM are different days")

	// Verify two points records exist now
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionDailyAttempt)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 2, "Should have two records: one from yesterday and one from today")
}

// TestGrantWeeklyLoginPoints_MidnightBoundary tests that weekly login properly counts distinct calendar days
func TestGrantWeeklyLoginPoints_MidnightBoundary(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Get today's date
	year, month, day := now.Date()

	// Create login events for 7 consecutive days, but with times near midnight
	// Day 0 (today): 00:01 AM
	createLoginEvent(t, client, userID, time.Date(year, month, day, 0, 1, 0, 0, now.Location()))

	// Day -1: 23:59 PM
	createLoginEvent(t, client, userID, time.Date(year, month, day-1, 23, 59, 0, 0, now.Location()))

	// Day -2: 00:30 AM
	createLoginEvent(t, client, userID, time.Date(year, month, day-2, 0, 30, 0, 0, now.Location()))

	// Day -3: 23:30 PM
	createLoginEvent(t, client, userID, time.Date(year, month, day-3, 23, 30, 0, 0, now.Location()))

	// Day -4: 01:00 AM
	createLoginEvent(t, client, userID, time.Date(year, month, day-4, 1, 0, 0, 0, now.Location()))

	// Day -5: 22:00 PM
	createLoginEvent(t, client, userID, time.Date(year, month, day-5, 22, 0, 0, 0, now.Location()))

	// Day -6: 02:00 AM
	createLoginEvent(t, client, userID, time.Date(year, month, day-6, 2, 0, 0, 0, now.Location()))

	// Grant weekly login points
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.True(t, granted, "Should grant weekly points when user logged in on 7 distinct calendar days, regardless of time")

	// Verify points were created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 1)
	require.Equal(t, events.PointValueWeeklyLogin, pointsRecords[0].Points)
}

// TestGrantWeeklyLoginPoints_NotEnoughDistinctDays tests that multiple logins on the same day don't count as multiple days
func TestGrantWeeklyLoginPoints_NotEnoughDistinctDays(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client, nil)
	userID := setupTestData(t, client)

	ctx := context.Background()
	now := time.Now()

	// Get today's date
	year, month, day := now.Date()

	// Create multiple login events on 6 days (not enough for weekly)
	for i := 0; i < 6; i++ {
		// Create 3 events per day at different times to test that multiple events on the same day only count once
		createLoginEvent(t, client, userID, time.Date(year, month, day-i, 8, 0, 0, 0, now.Location()))
		createLoginEvent(t, client, userID, time.Date(year, month, day-i, 16, 0, 0, 0, now.Location()))
		createLoginEvent(t, client, userID, time.Date(year, month, day-i, 23, 59, 59, 0, now.Location()))
	}

	// Also add multiple events just after midnight on day -5 (last day in our range)
	// These should still count as the same day (day -5), not a new day
	createLoginEvent(t, client, userID, time.Date(year, month, day-5, 0, 0, 1, 0, now.Location()))
	createLoginEvent(t, client, userID, time.Date(year, month, day-5, 0, 30, 0, 0, now.Location()))

	// Attempt to grant weekly login points
	granted, err := granter.GrantWeeklyLoginPoints(ctx, userID)
	require.NoError(t, err)
	require.False(t, granted, "Should not grant weekly points with only 6 distinct days (day-0 to day-5)")

	// Verify no points record was created
	pointsRecords, err := client.Point.Query().
		Where(point.HasUserWith(user.IDEQ(userID))).
		Where(point.DescriptionEQ(events.PointDescriptionWeeklyLogin)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, pointsRecords, 0)
}
