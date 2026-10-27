// Package testdb provides isolated PostgreSQL databases for integration tests.
//
// One pgvector container is started per test binary. Migrations run once into
// a template database; every test gets its own clone, so tests can run in
// parallel without sharing state.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/dev-fritz/newsp/api/internal/database"
)

// Image is the PostgreSQL image used by tests; it matches docker-compose.yml.
const Image = "pgvector/pgvector:0.8.7-pg17-bookworm"

const templateDB = "newsp_template"

var (
	once     sync.Once
	adminURL string // connection to the maintenance "postgres" database
	setupErr error
	counter  atomic.Int64

	// Postgres refuses concurrent copies of the same template database.
	createMu sync.Mutex
)

// New returns a fresh database with every migration applied.
func New(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := create(t, "TEMPLATE "+templateDB)
	return db
}

// NewEmpty returns a fresh database without migrations, plus its connection
// URL, for tests that run the migrations themselves.
func NewEmpty(t *testing.T) (*sql.DB, string) {
	t.Helper()
	return create(t, "")
}

func create(t *testing.T, clause string) (*sql.DB, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	once.Do(start)
	if setupErr != nil {
		t.Fatalf("start test database: %v", setupErr)
	}

	ctx := context.Background()
	name := fmt.Sprintf("test_%d_%d", os.Getpid(), counter.Add(1))

	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()
	createMu.Lock()
	_, err = admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s %s", name, clause))
	createMu.Unlock()
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	dbURL := withDatabase(adminURL, name)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		cleanup, err := sql.Open("pgx", adminURL)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	})
	return db, dbURL
}

func start() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, Image,
		postgres.WithDatabase(templateDB),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		setupErr = fmt.Errorf("run container: %w", err)
		return
	}
	connURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		setupErr = fmt.Errorf("connection string: %w", err)
		return
	}

	// The migrator closes its own connection, leaving the template free to copy.
	if err := database.MigrateUp(connURL); err != nil {
		setupErr = err
		return
	}
	adminURL = withDatabase(connURL, "postgres")
}

func withDatabase(raw, name string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Path = "/" + name
	return u.String()
}
