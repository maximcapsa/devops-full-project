package db

import "embed"

// MigrationsFS holds the golang-migrate SQL files, embedded so the binary
// carries its own schema and runs migrations on startup.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
