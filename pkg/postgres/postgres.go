// Package postgres provides a pgx connection pool with startup retry and a
// golang-migrate runner. The single database is shared schema-per-service:
// each service sets its search_path to its own schema, so migrations and
// queries use unqualified names and the per-service version table lives in
// that schema (no collisions between services).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/maximcapsa/devops-full-project/pkg/retry"
)

// Connect opens a pgx pool, retrying until the database is reachable (Postgres
// in compose/k8s may start after the service). If schema is non-empty the
// connection's search_path is pinned to it.
func Connect(ctx context.Context, url, schema string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	if schema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}

	var pool *pgxpool.Pool
	err = retry.Do(ctx, 12, 500*time.Millisecond, 5*time.Second, func() error {
		p, perr := pgxpool.NewWithConfig(ctx, cfg)
		if perr != nil {
			return perr
		}
		if perr := p.Ping(ctx); perr != nil {
			p.Close()
			return perr
		}
		pool = p
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return pool, nil
}

// Migrate applies the embedded migrations in fsys under dir to schema. The
// schema is created if missing, then search_path is pinned so both the DDL and
// the schema_migrations version table land inside it.
func Migrate(ctx context.Context, url string, fsys fs.FS, dir, schema string) error {
	connCfg, err := pgx.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}

	// Ensure the schema exists before pinning search_path to it.
	if schema != "" {
		bootstrap := stdlib.OpenDB(*connCfg)
		_, err = bootstrap.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgx.Identifier{schema}.Sanitize()))
		_ = bootstrap.Close()
		if err != nil {
			return fmt.Errorf("create schema %q: %w", schema, err)
		}
		connCfg.RuntimeParams["search_path"] = schema
	}

	db := stdlib.OpenDB(*connCfg)
	defer func() { _ = db.Close() }()

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{MigrationsTable: "schema_migrations"})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
