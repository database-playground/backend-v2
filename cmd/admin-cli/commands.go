package main

import (
	"context"
	"fmt"

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
			fmt.Printf("We have created a default administrator scope set (%s) with all scopes accessible,\n", result.AdminScopeSet.Slug)
			fmt.Printf("a default new-user scope set (%s) with the minimal scope set,\n", result.NewUserScopeSet.Slug)
			fmt.Printf("and a default unverified scope set (%s) with the minimal scope set.\n", result.UnverifiedScopeSet.Slug)
			fmt.Printf("Besides, we also created a default administrator group (%s, ID #%d) with the admin scope set,\n", result.AdminGroup.Name, result.AdminGroup.ID)
			fmt.Printf("a default new-user group (%s, ID #%d) with the minimal scope set,\n", result.StudentGroup.Name, result.StudentGroup.ID)
			fmt.Printf("and a default unverified group (%s, ID #%d) with the minimal scope set.\n", result.UnverifiedGroup.Name, result.UnverifiedGroup.ID)
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

func newRootCommand(subcommands ...*cli.Command) *cli.Command {
	return &cli.Command{
		Name:     "admin-cli",
		Usage:    "A CLI tool for managing the Database Playground instance.",
		Commands: subcommands,
	}
}
