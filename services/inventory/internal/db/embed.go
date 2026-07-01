package db

import "embed"

// MigrationsFS holds the golang-migrate SQL files for the inventory schema.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
