package events

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/posthog/posthog-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("dbplay.events")

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
	ctx, span := tracer.Start(ctx, "TriggerEvent",
		trace.WithAttributes(
			attribute.String("event.type", string(event.Type)),
			attribute.Int("user.id", event.UserID),
		))
	defer span.End()

	err := s.triggerEvent(ctx, event)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to trigger event")
		span.RecordError(err)
		slog.Error("failed to trigger event", "error", err)
	} else {
		span.SetStatus(otelcodes.Ok, "Event triggered successfully")
	}

	if s.posthogClient != nil {
		span.AddEvent("posthog.send", trace.WithAttributes(
			attribute.String("event.type", string(event.Type)),
			attribute.Int("user.id", event.UserID),
		))
		slog.Debug("sending event to PostHog", "event_type", event.Type, "user_id", event.UserID)
		err = s.posthogClient.Enqueue(posthog.Capture{
			DistinctId: strconv.Itoa(event.UserID),
			Event:      string(event.Type),
			Timestamp:  time.Now(),
			Properties: event.Payload,
		})
		if err != nil {
			span.RecordError(err)
			slog.Error("failed to send event to PostHog", "error", err)
		}
	}
}

// triggerEvent triggers an event synchronously.
func (s *EventService) triggerEvent(ctx context.Context, event Event) error {
	ctx, span := tracer.Start(ctx, "triggerEvent",
		trace.WithAttributes(
			attribute.String("event.type", string(event.Type)),
			attribute.Int("user.id", event.UserID),
		))
	defer span.End()

	span.AddEvent("database.event.create")
	eventEntity, err := s.entClient.Event.Create().
		SetType(string(event.Type)).
		SetPayload(event.Payload).
		SetUserID(event.UserID).
		SetTriggeredAt(time.Now()).
		Save(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to create event")
		span.RecordError(err)
		return err
	}

	span.SetAttributes(attribute.Int("event.id", eventEntity.ID))

	span.AddEvent("handlers.processing", trace.WithAttributes(
		attribute.Int("handlers.count", len(s.handlers)),
	))

	for i, handler := range s.handlers {
		span.AddEvent("handler.executing", trace.WithAttributes(
			attribute.Int("handler.index", i),
		))
		err := handler.HandleEvent(ctx, eventEntity)
		if err != nil {
			span.SetStatus(otelcodes.Error, "Failed to handle event")
			span.RecordError(err)
			return err
		}
	}

	span.SetStatus(otelcodes.Ok, "Event triggered successfully")
	return nil
}
