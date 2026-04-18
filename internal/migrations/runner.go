package migrations

import (
	"fmt"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4"
)

// RunMigrations applies all pending migrations from the migrations directory.
// It takes a PostgreSQL connection URL and runs all migrations in sequence.
// If no migrations are pending, it returns nil without error.
//
// Migration files are auto-discovered from the migrations/ directory.
// The golang-migrate library automatically sorts and executes migrations
// in lexicographic order (e.g., 001_*, 002_*, etc.).
func RunMigrations(dbURL string) error {
	// Path to migration files (relative to where the binary is executed)
	migrationPath := "file://migrations"

	// Initialize the migration instance
	m, err := migrate.New(migrationPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}
	defer m.Close()

	// Apply all pending migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
