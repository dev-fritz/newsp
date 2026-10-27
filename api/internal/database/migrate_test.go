package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

var expectedTables = []string{
	"users", "user_tokens", "refresh_tokens",
	"ai_providers", "ai_models", "instance_settings", "ai_usage",
	"sources", "items", "user_preferences", "user_source_exclusions",
	"reports", "report_items", "ideas", "idea_versions", "idea_evidence", "jobs",
}

func TestMigrationsUpDownUp(t *testing.T) {
	t.Parallel()
	db, url := testdb.NewEmpty(t)
	ctx := context.Background()

	listTables := func() []string {
		rows, err := db.QueryContext(ctx, `
			SELECT table_name FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		var names []string
		for rows.Next() {
			var n string
			require.NoError(t, rows.Scan(&n))
			names = append(names, n)
		}
		require.NoError(t, rows.Err())
		return names
	}

	require.NoError(t, database.MigrateUp(url))
	assert.ElementsMatch(t, expectedTables, listTables())

	m, err := database.NewMigrator(url)
	require.NoError(t, err)
	require.NoError(t, m.Down())
	_, _ = m.Close()
	assert.Empty(t, listTables())

	require.NoError(t, database.MigrateUp(url))
	assert.ElementsMatch(t, expectedTables, listTables())

	var settings int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM instance_settings`).Scan(&settings))
	assert.Equal(t, 1, settings, "the singleton settings row is seeded by the migration")
}
