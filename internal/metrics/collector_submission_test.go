package metrics

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/ent/submission"
	"github.com/database-playground/backend-v2/internal/testhelper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestData(t *testing.T, client *ent.Client) (*ent.User, *ent.Question) {
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

	// Create a test database
	testDatabase, err := client.Database.Create().
		SetSlug("test-db").
		SetDescription("Test database").
		SetSchema("CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT);").
		SetRelationFigure("test-figure").
		Save(ctx)
	require.NoError(t, err)

	// Create a test question
	testQuestion, err := client.Question.Create().
		SetCategory("basic-sql").
		SetDifficulty("easy").
		SetTitle("Select all users").
		SetDescription("Write a SQL query to select all users from the users table").
		SetReferenceAnswer("SELECT * FROM users;").
		SetDatabase(testDatabase).
		Save(ctx)
	require.NoError(t, err)

	return testUser, testQuestion
}

func TestSubmissionCollector_Collect(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	user, question := setupTestData(t, client)
	ctx := context.Background()

	// Create submissions with different statuses
	// 3 pending submissions
	for range 3 {
		_, err := client.Submission.Create().
			SetSubmittedCode("SELECT * FROM users;").
			SetStatus(submission.StatusPending).
			SetUser(user).
			SetQuestion(question).
			Save(ctx)
		require.NoError(t, err)
	}

	// 5 success submissions
	for range 5 {
		_, err := client.Submission.Create().
			SetSubmittedCode("SELECT * FROM users;").
			SetStatus(submission.StatusSuccess).
			SetUser(user).
			SetQuestion(question).
			Save(ctx)
		require.NoError(t, err)
	}

	// 2 failed submissions
	for range 2 {
		_, err := client.Submission.Create().
			SetSubmittedCode("SELECT * FROM invalid;").
			SetStatus(submission.StatusFailed).
			SetUser(user).
			SetQuestion(question).
			Save(ctx)
		require.NoError(t, err)
	}

	// Create collector
	collector := NewSubmissionCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	metrics, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	metric := metrics[0]
	assert.Equal(t, "dbplay_submissions_total", *metric.Name)
	assert.Equal(t, "Total number of submissions", *metric.Help)

	// Verify metric values
	metricMap := make(map[string]float64)
	for _, m := range metric.Metric {
		if len(m.Label) == 1 && m.Label[0].Name != nil && m.Label[0].Value != nil {
			status := *m.Label[0].Value
			value := *m.Gauge.Value
			metricMap[status] = value
		}
	}

	// Verify counts match expected values
	assert.Equal(t, 3.0, metricMap["pending"], "pending submissions count should be 3")
	assert.Equal(t, 5.0, metricMap["success"], "success submissions count should be 5")
	assert.Equal(t, 2.0, metricMap["failed"], "failed submissions count should be 2")
}

func TestSubmissionCollector_Collect_EmptyDatabase(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	setupTestData(t, client) // Setup but don't create any submissions

	// Create collector
	collector := NewSubmissionCollector(client)

	// Register collector and gather metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// When there are no submissions, Collect should not error
	// It simply won't send any metrics
	metrics, err := registry.Gather()
	require.NoError(t, err, "Gather should not error even with empty database")

	// With no submissions, no metric values should be collected
	require.Empty(t, metrics, "should have no metrics when database is empty")
}

func TestSubmissionCollector_Describe(t *testing.T) {
	client := testhelper.NewEntSqliteClient(t)
	collector := NewSubmissionCollector(client)

	// Create a channel to receive descriptions
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	// Verify description was sent
	desc := <-ch
	require.NotNil(t, desc)
	// Check that the description contains the metric name
	assert.Contains(t, desc.String(), "dbplay_submissions_total")
}
