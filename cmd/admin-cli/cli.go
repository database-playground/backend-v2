package main

import (
	"context"
	"log"
	"os"

	dpcli "github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/internal/deps"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := deps.Config()
	if err != nil {
		log.Fatal(err)
	}

	entClient, err := deps.EntClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	c := dpcli.NewContext(entClient)

	promoteAdminCommand := newPromoteAdminCommand(c)
	setupCommand := newSetupCommand(c)
	migrateCommand := newMigrateCommand(c)
	seedUsersCommand := newSeedUsersCommand(c)

	rootCommand := newRootCommand(promoteAdminCommand, setupCommand, migrateCommand, seedUsersCommand)

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
