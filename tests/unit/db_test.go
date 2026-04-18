package unit

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// setupTestDB initializes a test database connection.
func setupTestDB(t *testing.T) *sql.DB {
	// Use environment variables for test database connection
	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("skipping test: could not open test database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("skipping test: could not connect to test database: %v", err)
	}

	return db
}

// runMigrations applies all migrations from the migrations directory.
func runMigrations(t *testing.T, db *sql.DB) {
	// Get the absolute path to the migrations directory
	migrationPath, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("failed to get migration path: %v", err)
	}

	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"
	m, err := migrate.New("file://"+migrationPath, connStr)
	if err != nil {
		t.Fatalf("failed to initialize migrations: %v", err)
	}
	defer m.Close()

	// Apply all pending migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}
}

// TestGUISessionFieldsMigration verifies that the GUI session fields migration
// creates the necessary columns and indexes.
func TestGUISessionFieldsMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Run migrations
	runMigrations(t, db)

	// Verify gate_sessions table has GUI columns
	var enableGUI sql.NullBool
	var guiProtocol sql.NullString
	var xDisplay sql.NullInt64
	var vncPort sql.NullInt64
	query := `SELECT enable_gui, gui_protocol, x11_display_port, vnc_port
	          FROM gate_sessions LIMIT 1`
	err := db.QueryRow(query).Scan(&enableGUI, &guiProtocol, &xDisplay, &vncPort)
	// Query may fail (no rows), but shouldn't error on missing columns
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("columns missing: %v", err)
	}

	// Verify timestamp columns exist
	var startedAt, endedAt sql.NullTime
	query = `SELECT gui_session_started_at, gui_session_ended_at
	         FROM gate_sessions LIMIT 1`
	err = db.QueryRow(query).Scan(&startedAt, &endedAt)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("timestamp columns missing: %v", err)
	}

	// Verify index exists on gui_protocol
	var indexName sql.NullString
	query = `SELECT indexname FROM pg_indexes
	         WHERE tablename = 'gate_sessions'
	         AND indexname = 'idx_sessions_gui_protocol'`
	err = db.QueryRow(query).Scan(&indexName)
	if err != nil {
		t.Fatalf("gui_protocol index not created: %v", err)
	}
	if indexName.String != "idx_sessions_gui_protocol" {
		t.Fatalf("expected index name 'idx_sessions_gui_protocol', got '%s'", indexName.String)
	}
}

// TestGUISessionFieldsDefaults verifies that GUI fields have correct defaults.
func TestGUISessionFieldsDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Run migrations
	runMigrations(t, db)

	// Insert a minimal session to verify defaults
	query := `INSERT INTO gate_sessions
	          (user_id, device_id, state, started_at)
	          VALUES
	          ('550e8400-e29b-41d4-a716-446655440000',
	           '550e8400-e29b-41d4-a716-446655440001',
	           'active',
	           NOW())
	          RETURNING enable_gui, gui_protocol`

	var enableGUI sql.NullBool
	var guiProtocol sql.NullString

	err := db.QueryRow(query).Scan(&enableGUI, &guiProtocol)
	if err == sql.ErrNoRows {
		t.Skip("skipping: cannot insert device for test")
	}
	if err != nil {
		t.Fatalf("failed to insert test session: %v", err)
	}

	// Verify enable_gui defaults to false
	if !enableGUI.Valid {
		t.Fatal("enable_gui should not be NULL")
	}
	if enableGUI.Bool {
		t.Fatal("enable_gui should default to FALSE")
	}

	// Verify gui_protocol defaults to NULL
	if guiProtocol.Valid {
		t.Fatal("gui_protocol should be NULL")
	}
}
