// Command migrate applies or reverts the database migrations.
//
// Usage: migrate up | down | version
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/joho/godotenv"

	"github.com/dev-fritz/newsp/api/internal/config"
	"github.com/dev-fritz/newsp/api/internal/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: migrate up|down|version")
	}
	_ = godotenv.Load()

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	m, err := database.NewMigrator(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1) // one step at a time; dropping everything must be deliberate
	case "version":
		v, dirty, verr := m.Version()
		if errors.Is(verr, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied")
			return nil
		}
		if verr != nil {
			return verr
		}
		fmt.Printf("version %d (dirty: %t)\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q: use up, down or version", args[0])
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no change")
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate %s: %w", args[0], err)
	}
	fmt.Println("done")
	return nil
}
