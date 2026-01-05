package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	dpcli "github.com/database-playground/backend-v2/cli"
	"github.com/urfave/cli/v3"
)

func newMigrateCommand(clictx *dpcli.Context) *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Migrate the database to the latest version",
		Action: func(ctx context.Context, c *cli.Command) error {
			fmt.Println("Migrating the database to the latest version…")
			if err := clictx.Migrate(ctx); err != nil {
				return err
			}

			fmt.Println("✅ Migration complete!")
			return nil
		},
	}
}

func newSetupCommand(clictx *dpcli.Context) *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Setup the Database Playground instance",
		Action: func(ctx context.Context, c *cli.Command) error {
			fmt.Println("Setting up the Database Playground instance…")

			result, err := clictx.Setup(ctx)
			if err != nil {
				return err
			}

			fmt.Println("✅ Setup complete!")
			fmt.Println()
			fmt.Printf("%+v\n", result)
			fmt.Println()
			fmt.Println("You can then use the following commands to complete the setup:")
			fmt.Println("  - \"promote-admin\" to promote a user to an administrator account after registration.")
			fmt.Println()
			fmt.Println("For further migrations, you can use the following commands:")
			fmt.Println("  - \"migrate\" to migrate the database to the latest version.")

			return nil
		},
	}
}

func newPromoteAdminCommand(dpcli *dpcli.Context) *cli.Command {
	return &cli.Command{
		Name:  "promote-admin",
		Usage: "Promote a user to an administrator account",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "email",
				Usage:    "The email of the user to promote.",
				Required: true,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			email := c.String("email")
			fmt.Println("Promoting user", email, "to an administrator account.")

			if err := dpcli.PromoteAdmin(ctx, email); err != nil {
				return err
			}

			fmt.Println("✅ User", email, "has been promoted to an administrator.")

			return nil
		},
	}
}

func newSeedUsersCommand(clictx *dpcli.Context) *cli.Command {
	return &cli.Command{
		Name:        "seed-users",
		Usage:       "Seed the users table from a JSON file",
		Description: "Seed the users table from a JSON file. It should be a list with `{email: string, group?: string}` pairs. If group is omitted, the user will be added to the `student` group.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Usage:    "The JSON file to seed the database with users from.",
				Required: true,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			content, err := os.ReadFile(c.String("file"))
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			fmt.Printf("Seeding the database with users from %q…\n", c.String("file"))

			var userSeedRecords []dpcli.UserSeedRecord
			if err := json.Unmarshal(content, &userSeedRecords); err != nil {
				return fmt.Errorf("unmarshal users seed records: %w", err)
			}

			if err := clictx.SeedUsers(ctx, userSeedRecords); err != nil {
				return err
			}

			fmt.Println("✅ Users seeded!")
			return nil
		},
	}
}

func newRootCommand(subcommands ...*cli.Command) *cli.Command {
	return &cli.Command{
		Name:     "admin-cli",
		Usage:    "A CLI tool for managing the Database Playground instance.",
		Commands: subcommands,
	}
}
