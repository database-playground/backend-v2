package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDataForCheatRecord(t *testing.T, client *ent.Client) *ent.User {
	t.Helper()

	ctx := context.Background()

	// Create a scopeset
	scopeset, err := client.ScopeSet.Create().
		SetSlug("test").
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Create a group
	group, err := client.Group.Create().
		SetName("test").
		AddScopeSets(scopeset).
		Save(ctx)
	require.NoError(t, err)

	// Create a user for testing
	testUser, err := client.User.Create().
		SetName("Test User").
		SetEmail("test@example.com").
		SetGroup(group).
		Save(ctx)
	require.NoError(t, err)

	return testUser
}

func TestCheatRecordCollector_Collect(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	user := setupTestDataForCheatRecord(t, client)
	ctx := context.Background()

	// Create 5 unresolved cheat records
	for range 5 {
		_, err := client.CheatRecord.Create().
			SetReason("Suspicious activity detected").
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// Create 3 resolved cheat records
	for range 3 {
		_, err := client.CheatRecord.Create().
			SetReason("Suspicious activity detected").
			SetResolvedReason("False positive").
			SetResolvedAt(time.Now()).
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// Create collector
	collector := NewCheatRecordCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	metrics, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 2, "should have 2 metrics")

	// Find the metrics by name
	var totalMetric, resolvedMetric *dto.MetricFamily
	for _, m := range metrics {
		if m.Name != nil {
			switch *m.Name {
			case "dbplay_cheat_records_total":
				totalMetric = m
			case "dbplay_resolved_cheat_records_total":
				resolvedMetric = m
			}
		}
	}

	// Verify total cheat records metric
	require.NotNil(t, totalMetric, "dbplay_cheat_records_total metric should exist")
	assert.Equal(t, "dbplay_cheat_records_total", *totalMetric.Name)
	assert.Equal(t, "Total number of cheat records", *totalMetric.Help)
	require.Len(t, totalMetric.Metric, 1)
	assert.Equal(t, 8.0, *totalMetric.Metric[0].Gauge.Value, "total cheat records count should be 8")

	// Verify resolved cheat records metric
	require.NotNil(t, resolvedMetric, "dbplay_resolved_cheat_records_total metric should exist")
	assert.Equal(t, "dbplay_resolved_cheat_records_total", *resolvedMetric.Name)
	assert.Equal(t, "Total number of resolved cheat records", *resolvedMetric.Help)
	require.Len(t, resolvedMetric.Metric, 1)
	assert.Equal(t, 3.0, *resolvedMetric.Metric[0].Gauge.Value, "resolved cheat records count should be 3")
}

func TestCheatRecordCollector_Collect_EmptyDatabase(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	setupTestDataForCheatRecord(t, client) // Setup but don't create any cheat records

	// Create collector
	collector := NewCheatRecordCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	metrics, err := registry.Gather()
	require.NoError(t, err, "Gather should not error even with empty database")
	require.Len(t, metrics, 2, "should have 2 metrics even with empty database")

	// Find the metrics by name
	var totalMetric, resolvedMetric *dto.MetricFamily
	for _, m := range metrics {
		if m.Name != nil {
			switch *m.Name {
			case "dbplay_cheat_records_total":
				totalMetric = m
			case "dbplay_resolved_cheat_records_total":
				resolvedMetric = m
			}
		}
	}

	// Verify both metrics exist and have zero values
	require.NotNil(t, totalMetric)
	require.Len(t, totalMetric.Metric, 1)
	assert.Equal(t, 0.0, *totalMetric.Metric[0].Gauge.Value, "total cheat records count should be 0")

	require.NotNil(t, resolvedMetric)
	require.Len(t, resolvedMetric.Metric, 1)
	assert.Equal(t, 0.0, *resolvedMetric.Metric[0].Gauge.Value, "resolved cheat records count should be 0")
}

func TestCheatRecordCollector_Collect_OnlyUnresolved(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	user := setupTestDataForCheatRecord(t, client)
	ctx := context.Background()

	// Create only unresolved cheat records
	for range 4 {
		_, err := client.CheatRecord.Create().
			SetReason("Suspicious activity detected").
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// Create collector
	collector := NewCheatRecordCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	metrics, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	// Find the metrics by name
	var totalMetric, resolvedMetric *dto.MetricFamily
	for _, m := range metrics {
		if m.Name != nil {
			switch *m.Name {
			case "dbplay_cheat_records_total":
				totalMetric = m
			case "dbplay_resolved_cheat_records_total":
				resolvedMetric = m
			}
		}
	}

	// Verify counts
	require.NotNil(t, totalMetric)
	assert.Equal(t, 4.0, *totalMetric.Metric[0].Gauge.Value, "total cheat records count should be 4")

	require.NotNil(t, resolvedMetric)
	assert.Equal(t, 0.0, *resolvedMetric.Metric[0].Gauge.Value, "resolved cheat records count should be 0")
}

func TestCheatRecordCollector_Describe(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	collector := NewCheatRecordCollector(client)

	// Create a channel to receive descriptions
	ch := make(chan *prometheus.Desc, 2)
	collector.Describe(ch)
	close(ch)

	// Verify descriptions were sent
	descs := make([]*prometheus.Desc, 0, 2)
	for desc := range ch {
		descs = append(descs, desc)
	}

	require.Len(t, descs, 2, "should describe 2 metrics")

	// Check that the descriptions contain the metric names
	descStrings := make([]string, len(descs))
	for i, desc := range descs {
		descStrings[i] = desc.String()
	}

	assert.Contains(t, descStrings[0]+descStrings[1], "dbplay_cheat_records_total")
	assert.Contains(t, descStrings[0]+descStrings[1], "dbplay_resolved_cheat_records_total")
}

func TestNewCheatRecordCollector(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	collector := NewCheatRecordCollector(client)

	require.NotNil(t, collector)
	assert.Equal(t, client, collector.entClient)
}
