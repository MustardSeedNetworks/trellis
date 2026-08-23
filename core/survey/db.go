// SPDX-License-Identifier: BUSL-1.1

package survey

// db.go opens and migrates the survey store.
//
// modernc.org/sqlite is the pure-Go driver — no cgo, so Trellis keeps building
// for every target with the plain toolchain, which matters for a field tool
// that ships as one binary. It is the same driver and the same goose migration
// discipline seed runs, so the two repos fail the same way and are debugged the
// same way.

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// openDB opens the survey database at path and brings the schema up to date.
//
// The DSN turns foreign keys ON for every pooled connection. SQLite defaults
// them OFF per connection, so a schema full of REFERENCES clauses enforces
// nothing unless this is set — the cascade from a deleted survey down to its
// samples is the whole reason the schema is shaped this way.
func openDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open survey db: %w", err)
	}
	if err := migrate(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// migrate applies the embedded migrations.
func migrate(conn *sql.DB) error {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, sub)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply survey migrations: %w", err)
	}
	return nil
}
