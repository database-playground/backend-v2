package events

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/posthog/posthog-go"
)

// EventService is the service for triggering events.
type EventService struct {
	entClient     *ent.Client
	posthogClient posthog.Client

	handlers []EventHandler
}

// NewEventService creates a new EventService.
func NewEventService(entClient *ent.Client, posthogClient posthog.Client) *EventService {
	return &EventService{
		entClient:     entClient,
		posthogClient: posthogClient,
		handlers:      []EventHandler{NewPointsGranter(entClient, posthogClient)},
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
	HandleEvent(ctx context.Context, event *ent.Event) error
}

// TriggerEvent triggers an event.
func (s *EventService) TriggerEvent(ctx context.Context, event Event) {
	err := s.triggerEvent(ctx, event)
	if err != nil {
		slog.Error("failed to trigger event", "error", err)
	}

	if s.posthogClient != nil {
		slog.Debug("sending event to PostHog", "event_type", event.Type, "user_id", event.UserID)
		err = s.posthogClient.Enqueue(posthog.Capture{
			DistinctId: strconv.Itoa(event.UserID),
			Event:      string(event.Type),
			Timestamp:  time.Now(),
			Properties: event.Payload,
		})
		if err != nil {
			slog.Error("failed to send event to PostHog", "error", err)
		}
	}
}

// triggerEvent triggers an event synchronously.
func (s *EventService) triggerEvent(ctx context.Context, event Event) error {
	eventEntity, err := s.entClient.Event.Create().
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
