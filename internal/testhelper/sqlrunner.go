package testhelper

import (
	"context"
	"testing"

	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func NewSQLRunnerClient(t *testing.T) *sqlrunner.SqlRunner {
	t.Helper()

	container := NewSQLRunnerContainer(t)

	endpoint, err := container.PortEndpoint(context.Background(), nat.Port("8080/tcp"), "http")
	if err != nil {
		t.Skipf("failed to get SQL Runner container endpoint: %v", err)
	}

	return sqlrunner.NewSqlRunner(config.SqlRunnerConfig{
		URI: endpoint,
	})
}

func NewSQLRunnerContainer(t *testing.T) testcontainers.Container {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/database-playground/sqlrunner-v2:main",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/healthz").WithPort("8080/tcp"),
	}
	sqlrunnerC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("failed to create SQL Runner container: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlrunnerC.Terminate(context.Background()); err != nil {
			t.Logf("failed to terminate SQL Runner container: %v", err)
		}
	})

	return sqlrunnerC
}
