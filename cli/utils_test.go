package cli_test

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/internal/testhelper"
)

func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	entClient := testhelper.NewEntSqliteClient(t)
	eventService := events.NewEventService(entClient, nil)
	sqlRunner := testhelper.NewSQLRunnerClient(t)

	return &TestContext{
		entClient:    entClient,
		eventService: eventService,
		sqlRunner:    sqlRunner,
	}
}

type TestContext struct {
	entClient    *ent.Client
	eventService *events.EventService
	sqlRunner    *sqlrunner.SqlRunner
}

func (tc *TestContext) Setup(t *testing.T) {
	t.Helper()

	cliContext := tc.GetContext(t)

	_, err := cliContext.Setup(context.Background())
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
}

func (tc *TestContext) GetContext(t *testing.T) *cli.Context {
	t.Helper()

	return cli.NewContext(tc.entClient, tc.eventService, tc.sqlRunner)
}

func (tc *TestContext) GetEntClient(t *testing.T) *ent.Client {
	t.Helper()

	return tc.entClient
}

func (tc *TestContext) GetEventService(t *testing.T) *events.EventService {
	t.Helper()

	return tc.eventService
}

func (tc *TestContext) GetSQLRunner(t *testing.T) *sqlrunner.SqlRunner {
	t.Helper()

	return tc.sqlRunner
}
