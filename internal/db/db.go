package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/Jenn2U/JennGate/internal/config"
)

// InitDB initializes a PostgreSQL database connection from the provided config.
// It builds the connection string, opens the connection, validates it, and
// configures connection pool parameters.
func InitDB(cfg *config.Config) (*sql.DB, error) {
	// Build PostgreSQL connection string
	connString := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	// Open database connection
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Validate connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return db, nil
}

// Close gracefully closes the database connection.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}
