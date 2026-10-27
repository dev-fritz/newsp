// Package migrations embeds the SQL migrations so cmd/migrate and the
// integration tests ship with them.
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
