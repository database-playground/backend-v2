package metrics

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/event"
	"github.com/prometheus/client_golang/prometheus"
	otelcodes "go.opentelemetry.io/otel/codes"
)

var dbplayEventsTotalDesc = prometheus.NewDesc(
	"dbplay_events_total",
	"Total number of events",
	[]string{"type"},
	nil,
)

type EventCollector struct {
	entClient *ent.Client
}

func NewEventCollector(entClient *ent.Client) *EventCollector {
	return &EventCollector{entClient: entClient}
}

func (c *EventCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbplayEventsTotalDesc
}

func (c *EventCollector) Collect(ch chan<- prometheus.Metric) {
	_, span := tracer.Start(context.Background(), "EventCollector.Collect")
	defer span.End()

	var results []struct {
		Type  string `json:"type,omitempty"`
		Count int    `json:"count,omitempty"`
	}

	err := c.entClient.Event.
		Query().
		GroupBy(event.FieldType).
		Aggregate(ent.Count()).
		Scan(context.Background(), &results)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to collect events")
		span.RecordError(err)

		ch <- prometheus.NewInvalidMetric(dbplayEventsTotalDesc, err)
		return
	}

	span.SetStatus(otelcodes.Ok, "Events collected successfully")

	for _, result := range results {
		ch <- prometheus.MustNewConstMetric(dbplayEventsTotalDesc, prometheus.GaugeValue, float64(result.Count), result.Type)
	}
}

var _ prometheus.Collector = (*EventCollector)(nil)
