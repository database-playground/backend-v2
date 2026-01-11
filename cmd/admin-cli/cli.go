package main

import (
	"context"
	"log"
	"os"

	dpcli "github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/deps"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/sqlrunner"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.LoadAdminCLIConfig()
	if err != nil {
		log.Fatal(err)
	}

	entClient, err := deps.EntClient(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	eventService := events.NewEventService(entClient, nil)
	sqlRunner := sqlrunner.NewSqlRunner(cfg.SqlRunner)
	c := dpcli.NewContext(entClient, eventService, sqlRunner)

	promoteAdminCommand := newPromoteAdminCommand(c)
	setupCommand := newSetupCommand(c)
	migrateCommand := newMigrateCommand(c)
	seedUsersCommand := newSeedUsersCommand(c)
	rerunAllSubmissionsCommand := newRerunAllSubmissionsCommand(c)

	rootCommand := newRootCommand(promoteAdminCommand, setupCommand, migrateCommand, seedUsersCommand, rerunAllSubmissionsCommand)

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
