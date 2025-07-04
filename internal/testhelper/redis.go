package testhelper

import (
	"context"
	"testing"

	"github.com/redis/rueidis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func NewRedisContainer(t *testing.T) testcontainers.Container {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("failed to create Redis container: %v", err)
	}

	t.Cleanup(func() {
		redisC.Terminate(context.Background())
	})

	return redisC
}

func NewRedisClient(t *testing.T, container testcontainers.Container) rueidis.Client {
	t.Helper()

	endpoint, err := container.Endpoint(context.Background(), "")
	if err != nil {
		t.Skipf("failed to get Redis container endpoint: %v", err)
	}

	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{endpoint},
	})
	if err != nil {
		t.Skipf("failed to create Redis client: %v", err)
	}

	t.Cleanup(func() {
		redisClient.Close()
	})

	return redisClient
}
