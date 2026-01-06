package metrics

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/prometheus/client_golang/prometheus"
	otelcodes "go.opentelemetry.io/otel/codes"
)

var dbplaySubmissionTotalDesc = prometheus.NewDesc(
	"dbplay_submissions_total",
	"Total number of submissions",
	[]string{"status"},
	nil,
)

type SubmissionCollector struct {
	entClient *ent.Client
}

func NewSubmissionCollector(entClient *ent.Client) *SubmissionCollector {
	return &SubmissionCollector{entClient: entClient}
}

func (c *SubmissionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbplaySubmissionTotalDesc
}

func (c *SubmissionCollector) Collect(ch chan<- prometheus.Metric) {
	_, span := tracer.Start(context.Background(), "SubmissionCollector.Collect")
	defer span.End()

	var results []struct {
		Status string `json:"status,omitempty"`
		Count  int    `json:"count,omitempty"`
	}

	err := c.entClient.Submission.
		Query().
		GroupBy(submission.FieldStatus).
		Aggregate(ent.Count()).
		Scan(context.Background(), &results)
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to collect submissions")
		span.RecordError(err)

		ch <- prometheus.NewInvalidMetric(dbplaySubmissionTotalDesc, err)
		return
	}

	span.SetStatus(otelcodes.Ok, "Submissions collected successfully")

	for _, result := range results {
		ch <- prometheus.MustNewConstMetric(dbplaySubmissionTotalDesc, prometheus.GaugeValue, float64(result.Count), result.Status)
	}
}

var _ prometheus.Collector = (*SubmissionCollector)(nil)
