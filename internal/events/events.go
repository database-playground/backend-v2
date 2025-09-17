package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/workers"
)

// EventService is the service for triggering events.
type EventService struct {
	entClient *ent.Client

	handlers []EventHandler
}

// NewEventService creates a new EventService.
func NewEventService(entClient *ent.Client) *EventService {
	return &EventService{
		entClient: entClient,
		handlers:  []EventHandler{NewPointsGranter(entClient)},
	}
}

// Event is the event to be triggered.
type Event struct {
	Type    EventType
	Payload map[string]any
	UserID  int
}

// EventHandler is the handler for the event.
//
// You can think it as the callback of the event.
type EventHandler interface {
	HandleEvent(ctx context.Context, event *ent.Events) error
}

// TriggerEvent triggers an event.
func (s *EventService) TriggerEvent(ctx context.Context, event Event) {
	workers.Global.Go(func() {
		err := s.triggerEvent(ctx, event)
		if err != nil {
			slog.Error("failed to trigger event", "error", err)
		}
	})
}

// triggerEvent triggers an event synchronously.
func (s *EventService) triggerEvent(ctx context.Context, event Event) error {
	eventEntity, err := s.entClient.Events.Create().
		SetType(string(event.Type)).
		SetPayload(event.Payload).
		SetUserID(event.UserID).
		SetTriggeredAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}

	for _, handler := range s.handlers {
		err := handler.HandleEvent(ctx, eventEntity)
		if err != nil {
			return err
		}
	}

	return nil
}
