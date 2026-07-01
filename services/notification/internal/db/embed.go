package db

import "embed"

// MigrationsFS holds the golang-migrate SQL files for the notification schema.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
