package metrics

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func setupEventTestData(t *testing.T, client *ent.Client) *ent.User {
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

func TestEventCollector_Collect(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	user := setupEventTestData(t, client)
	ctx := context.Background()

	// Create events with different types
	// 3 login events
	for range 3 {
		_, err := client.Event.Create().
			SetType("login").
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// 5 submit_answer events
	for range 5 {
		_, err := client.Event.Create().
			SetType("submit_answer").
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// 2 logout events
	for range 2 {
		_, err := client.Event.Create().
			SetType("logout").
			SetUser(user).
			Save(ctx)
		require.NoError(t, err)
	}

	// Create collector
	collector := NewEventCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	metrics, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	metric := metrics[0]
	assert.Equal(t, "dbplay_events_total", *metric.Name)
	assert.Equal(t, "Total number of events", *metric.Help)

	// Verify metric values
	metricMap := make(map[string]float64)
	for _, m := range metric.Metric {
		if len(m.Label) == 1 && m.Label[0].Name != nil && m.Label[0].Value != nil {
			eventType := *m.Label[0].Value
			value := *m.Gauge.Value
			metricMap[eventType] = value
		}
	}

	// Verify counts match expected values
	assert.Equal(t, 3.0, metricMap["login"], "login events count should be 3")
	assert.Equal(t, 5.0, metricMap["submit_answer"], "submit_answer events count should be 5")
	assert.Equal(t, 2.0, metricMap["logout"], "logout events count should be 2")
}

func TestEventCollector_Collect_EmptyDatabase(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	setupEventTestData(t, client) // Setup but don't create any events

	// Create collector
	collector := NewEventCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// When there are no events, Collect should not error
	// It simply won't send any metrics
	metrics, err := registry.Gather()
	require.NoError(t, err, "Gather should not error even with empty database")

	// With no events, no metric values should be collected
	require.Empty(t, metrics, "should have no metrics when database is empty")
}

func TestEventCollector_Describe(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	collector := NewEventCollector(client)

	// Create a channel to receive descriptions
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	// Verify description was sent
	desc := <-ch
	require.NotNil(t, desc)
	// Check that the description contains the metric name
	assert.Contains(t, desc.String(), "dbplay_events_total")
}
