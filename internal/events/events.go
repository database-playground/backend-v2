package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/database-playground/backend-v2/ent"
)

// EventService is the service for triggering events.
type EventService struct {
	entClient     *ent.Client
	pointsGranter *PointsGranter
}

// NewEventService creates a new EventService.
func NewEventService(entClient *ent.Client) *EventService {
	return &EventService{
		entClient:     entClient,
		pointsGranter: NewPointsGranter(entClient),
	}
}

// Event is the event to be triggered.
type Event struct {
	Type    EventType
	Payload map[string]any
	UserID  int
}

// TriggerEvent triggers an event.
func (s *EventService) TriggerEvent(ctx context.Context, event Event) error {
	err := s.entClient.Events.Create().
		SetType(string(event.Type)).
		SetPayload(event.Payload).
		SetUserID(event.UserID).
		SetTriggeredAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}

	if event.Type == EventTypeLogin {
		ok, err := s.pointsGranter.GrantDailyLoginPoints(ctx, event.UserID)
		if err != nil {
			return err
		}
		if ok {
			slog.Info("granted daily login points", "user_id", event.UserID)
		}
	}

	return nil
}
