package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/dev-fritz/newsp/api/migrations"
)

// NewMigrator builds a migrator over the embedded SQL migrations, with its own
// connection pool: the migrate driver closes the *sql.DB it is given, so it must
// never share the application's pool. Callers must Close the migrator.
//
// Migrations run only through cmd/migrate, never on API startup.
func NewMigrator(url string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open migration connection: %w", err)
	}
	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("migrator: %w", err)
	}
	return m, nil
}

// MigrateUp applies every pending migration.
func MigrateUp(url string) error {
	m, err := NewMigrator(url)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
