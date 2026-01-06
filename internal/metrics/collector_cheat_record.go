package metrics

import (
	"context"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/cheatrecord"
	"github.com/prometheus/client_golang/prometheus"
	otelcodes "go.opentelemetry.io/otel/codes"
)

var dbplayCheatRecordsTotalDesc = prometheus.NewDesc(
	"dbplay_cheat_records_total",
	"Total number of cheat records",
	nil,
	nil,
)

var dbplayResolvedCheatRecordsTotalDesc = prometheus.NewDesc(
	"dbplay_resolved_cheat_records_total",
	"Total number of resolved cheat records",
	nil,
	nil,
)

type CheatRecordCollector struct {
	entClient *ent.Client
}

func NewCheatRecordCollector(entClient *ent.Client) *CheatRecordCollector {
	return &CheatRecordCollector{entClient: entClient}
}

func (c *CheatRecordCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbplayCheatRecordsTotalDesc
	ch <- dbplayResolvedCheatRecordsTotalDesc
}

func (c *CheatRecordCollector) Collect(ch chan<- prometheus.Metric) {
	_, span := tracer.Start(context.Background(), "CheatRecordCollector.Collect")
	defer span.End()

	total, err := c.entClient.CheatRecord.Query().Count(context.Background())
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to collect cheat records")
		span.RecordError(err)

		ch <- prometheus.NewInvalidMetric(dbplayCheatRecordsTotalDesc, err)
		return
	}
	ch <- prometheus.MustNewConstMetric(dbplayCheatRecordsTotalDesc, prometheus.GaugeValue, float64(total))

	resolvedTotal, err := c.entClient.CheatRecord.Query().Where(cheatrecord.ResolvedAtNotNil()).Count(context.Background())
	if err != nil {
		span.SetStatus(otelcodes.Error, "Failed to collect resolved cheat records")
		span.RecordError(err)

		ch <- prometheus.NewInvalidMetric(dbplayResolvedCheatRecordsTotalDesc, err)
		return
	}
	ch <- prometheus.MustNewConstMetric(dbplayResolvedCheatRecordsTotalDesc, prometheus.GaugeValue, float64(resolvedTotal))

	span.SetStatus(otelcodes.Ok, "Cheat records collected successfully")
}

var _ prometheus.Collector = (*CheatRecordCollector)(nil)
