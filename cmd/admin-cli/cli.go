package main

import (
	"context"
	"log"
	"os"

	dpcli "github.com/database-playground/backend-v2/cli"
	"github.com/database-playground/backend-v2/internal/deps"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	entClient, err := deps.EntClient()
	if err != nil {
		log.Fatal(err)
	}

	c := dpcli.NewCliContext(entClient)

	promoteAdminCommand := newPromoteAdminCommand(c)
	setupCommand := newSetupCommand(c)
	migrateCommand := newMigrateCommand(c)

	rootCommand := newRootCommand(promoteAdminCommand, setupCommand, migrateCommand)

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
