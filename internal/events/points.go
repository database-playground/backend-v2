package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/events"
	"github.com/database-playground/backend-v2/ent/points"
	"github.com/database-playground/backend-v2/ent/user"
)

const (
	PointDescriptionDailyLogin  = "daily login"
	PointDescriptionWeeklyLogin = "weekly login"
)

const (
	PointValueDailyLogin  = 20
	PointValueWeeklyLogin = 50
)

// PointsGranter determines if the criteria is met to grant points to a user.
type PointsGranter struct {
	entClient *ent.Client
}

// NewPointsGranter creates a new PointsGranter.
func NewPointsGranter(entClient *ent.Client) *PointsGranter {
	return &PointsGranter{
		entClient: entClient,
	}
}

// HandleEvent handles the event creation.
func (d *PointsGranter) HandleEvent(ctx context.Context, event *ent.Events) error {
	switch event.Type {
	case string(EventTypeLogin):
		ok, err := d.GrantDailyLoginPoints(ctx, event.UserID)
		if ok {
			slog.Info("granted daily login points", "user_id", event.UserID)
		}
		return err
	}
	return nil
}

// GrantDailyLoginPoints grants the "daily login" points to a user.
func (d *PointsGranter) GrantDailyLoginPoints(ctx context.Context, userID int) (bool, error) {
	// Check if we have granted the "daily login" points for this user today.
	hasPointsRecord, err := d.entClient.Points.Query().
		Where(points.HasUserWith(user.ID(userID))).
		Where(points.DescriptionEQ(PointDescriptionDailyLogin)).
		Where(points.GrantedAtGTE(time.Now().AddDate(0, 0, -1))).Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has logged in today.
	hasTodayLoginRecord, err := d.entClient.Events.Query().
		Where(events.Type(string(EventTypeLogin))).
		Where(events.UserID(userID)).
		Where(events.TriggeredAtGTE(time.Now().AddDate(0, 0, -1))).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	if !hasTodayLoginRecord {
		return false, nil
	}

	// Grant the "daily login" points to the user.
	err = d.entClient.Points.Create().
		SetUserID(userID).
		SetDescription(PointDescriptionDailyLogin).
		SetPoints(PointValueDailyLogin).
		Exec(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GrantWeeklyLoginPoints grants the "weekly login" points to a user.
func (d *PointsGranter) GrantWeeklyLoginPoints(ctx context.Context, userID int) (bool, error) {
	// Check if we have granted the "weekly login" points for this user this week.
	hasPointsRecord, err := d.entClient.Points.Query().
		Where(points.HasUserWith(user.ID(userID))).
		Where(points.DescriptionEQ(PointDescriptionWeeklyLogin)).
		Where(points.GrantedAtGTE(time.Now().AddDate(0, 0, -7))).Exist(ctx)
	if err != nil {
		return false, err
	}
	if hasPointsRecord {
		return false, nil
	}

	// Check if the user has logged in every day this week.
	weekLoginRecords, err := d.entClient.Events.Query().
		Where(events.Type(string(EventTypeLogin))).
		Where(events.UserID(userID)).
		All(ctx)
	if err != nil {
		return false, err
	}

	// aggreated by day
	weekLoginRecordsByDay := make(map[time.Time]int)
	for _, record := range weekLoginRecords {
		weekLoginRecordsByDay[record.TriggeredAt.Truncate(24*time.Hour)]++
	}

	if len(weekLoginRecordsByDay) != 7 {
		return false, nil
	}

	// Grant the "weekly login" points to the user.
	err = d.entClient.Points.Create().
		SetUserID(userID).
		SetDescription(PointDescriptionWeeklyLogin).
		SetPoints(PointValueWeeklyLogin).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	return true, nil
}
