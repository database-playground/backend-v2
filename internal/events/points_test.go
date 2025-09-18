package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/point"
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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
			granter := events.NewPointsGranter(client)
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
	granter := events.NewPointsGranter(client)
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

	granter := events.NewPointsGranter(client)
	require.NotNil(t, granter)
}

func TestGrantDailyLoginPoints_NonExistentUser(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	granter := events.NewPointsGranter(client)

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
	granter := events.NewPointsGranter(client)

	ctx := context.Background()
	nonExistentUserID := 99999

	// Attempt to grant points for non-existent user should not fail during the query phase
	// but should return false since there are no login events
	granted, err := granter.GrantWeeklyLoginPoints(ctx, nonExistentUserID)
	require.NoError(t, err)
	require.False(t, granted)
}
