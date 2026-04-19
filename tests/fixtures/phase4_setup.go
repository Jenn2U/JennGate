package fixtures

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/Jenn2U/JennGate/internal/services"
)

// Phase4Setup wraps all services needed for Phase 4 integration tests.
// It provides a complete testing environment with database, all required services,
// and cleanup utilities.
type Phase4Setup struct {
	DB              *sql.DB
	SessionService  *services.SessionService
	PolicyService   *services.PolicyService
	CAService       *services.CAService
	TestContext     context.Context
	TestCancel      context.CancelFunc
}

// NewPhase4Setup initializes a complete Phase 4 test environment.
// It sets up the test database connection, initializes all services,
// and returns a ready-to-use test fixture.
func NewPhase4Setup(t *testing.T) *Phase4Setup {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Initialize test DB
	testDB := setupTestDB(t)

	// Initialize services with test DB
	sessionSvc := services.NewSessionService(testDB)
	policySvc := services.NewPolicyService()

	// CAService requires active CA key in database; initialize with error handling
	caSvc, err := services.NewCAService(testDB)
	if err != nil {
		t.Logf("Warning: CAService initialization failed (expected if no CA key in DB): %v", err)
		// Tests can create CA keys as needed
		caSvc = nil
	}

	return &Phase4Setup{
		DB:              testDB,
		SessionService:  sessionSvc,
		PolicyService:   policySvc,
		CAService:       caSvc,
		TestContext:     ctx,
		TestCancel:      cancel,
	}
}

// Teardown cleans up the Phase4Setup and closes database connections.
// Should be deferred in all tests using NewPhase4Setup.
func (ps *Phase4Setup) Teardown(t *testing.T) {
	t.Helper()
	ps.TestCancel()
	cleanupTestDB(t, ps.DB)
}

// ============================================================================
// Helper Functions for Phase 4 Test Setup
// ============================================================================

// CreateTestDevice creates a device in PENDING_APPROVAL state.
// Returns the device ID on success.
func (ps *Phase4Setup) CreateTestDevice(t *testing.T, deviceID, deviceType string) {
	t.Helper()

	query := `
		INSERT INTO devices (id, device_name, device_type, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`

	now := time.Now()
	_, err := ps.DB.ExecContext(
		ps.TestContext,
		query,
		deviceID, deviceID+"-name", deviceType, "PENDING_APPROVAL", now, now,
	)
	if err != nil {
		t.Fatalf("failed to create test device: %v", err)
	}
}

// ApproveTestDevice transitions a device from PENDING_APPROVAL to APPROVED.
func (ps *Phase4Setup) ApproveTestDevice(t *testing.T, deviceID, adminUserID string) {
	t.Helper()

	query := `
		UPDATE devices
		SET state = $1, approved_by = $2, approved_at = $3, updated_at = $4
		WHERE id = $5
	`

	now := time.Now()
	result, err := ps.DB.ExecContext(
		ps.TestContext,
		query,
		"APPROVED", adminUserID, now, now, deviceID,
	)
	if err != nil {
		t.Fatalf("failed to approve device: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		t.Fatalf("device not found: %s", deviceID)
	}
}

// CreateTestPolicy sets permissions for a user on a device.
func (ps *Phase4Setup) CreateTestPolicy(
	t *testing.T,
	userID string,
	deviceID string,
	permissions []string,
) {
	t.Helper()

	ps.PolicyService.SetPolicy(userID, deviceID, permissions)
}

// CreateTestSession creates a session in REQUESTED state.
// Returns the session object.
func (ps *Phase4Setup) CreateTestSession(
	t *testing.T,
	userID string,
	deviceID string,
	certSerial string,
) *services.Session {
	t.Helper()

	certExpiresAt := time.Now().Add(1 * time.Hour)
	session, err := ps.SessionService.CreateSession(
		ps.TestContext,
		userID,
		deviceID,
		certSerial,
		certExpiresAt,
	)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	return session
}

// TransitionSessionState transitions a session to a new state.
// Validates the state transition according to the session state machine.
func (ps *Phase4Setup) TransitionSessionState(
	t *testing.T,
	sessionID string,
	newState string,
) {
	t.Helper()

	err := ps.SessionService.UpdateSessionState(ps.TestContext, sessionID, newState)
	if err != nil {
		t.Fatalf("failed to transition session state to %s: %v", newState, err)
	}
}

// MarkSessionConnected transitions a session to ACTIVE state via MarkConnected.
func (ps *Phase4Setup) MarkSessionConnected(t *testing.T, sessionID string) {
	t.Helper()

	err := ps.SessionService.MarkConnected(ps.TestContext, sessionID)
	if err != nil {
		t.Fatalf("failed to mark session connected: %v", err)
	}
}

// DisconnectSession terminates a session with a disconnect reason.
func (ps *Phase4Setup) DisconnectSession(
	t *testing.T,
	sessionID string,
	reason string,
) {
	t.Helper()

	err := ps.SessionService.DisconnectSession(ps.TestContext, sessionID, reason)
	if err != nil {
		t.Fatalf("failed to disconnect session: %v", err)
	}
}

// GetSession retrieves a session by ID.
func (ps *Phase4Setup) GetSession(t *testing.T, sessionID string) *services.Session {
	t.Helper()

	session, err := ps.SessionService.GetSession(ps.TestContext, sessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	return session
}

// GetDevice retrieves a device by ID.
// Returns nil if device not found (use for existence checks).
func (ps *Phase4Setup) GetDevice(t *testing.T, deviceID string) *Device {
	t.Helper()

	query := `
		SELECT id, device_name, device_type, state, approved_by, approved_at, created_at, updated_at
		FROM devices
		WHERE id = $1
	`

	device := &Device{}
	err := ps.DB.QueryRowContext(ps.TestContext, query, deviceID).Scan(
		&device.ID, &device.DeviceName, &device.DeviceType, &device.State,
		&device.ApprovedBy, &device.ApprovedAt, &device.CreatedAt, &device.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	return device
}

// Device represents a test device record.
type Device struct {
	ID           string
	DeviceName   string
	DeviceType   string
	State        string
	ApprovedBy   *string
	ApprovedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AuditLog represents a test audit log entry.
type AuditLog struct {
	ID        string
	EventType string
	Actor     string
	Details   *string
	Timestamp time.Time
}

// GetAuditLogs retrieves all audit logs from the database.
func (ps *Phase4Setup) GetAuditLogs(t *testing.T) []AuditLog {
	t.Helper()

	query := `
		SELECT id, event_type, actor, details, created_at
		FROM gate_audit_log
		ORDER BY created_at DESC
	`

	rows, err := ps.DB.QueryContext(ps.TestContext, query)
	if err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		err := rows.Scan(&log.ID, &log.EventType, &log.Actor, &log.Details, &log.Timestamp)
		if err != nil {
			t.Fatalf("failed to scan audit log: %v", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating audit logs: %v", err)
	}

	return logs
}

// ============================================================================
// Database Setup & Cleanup Functions
// ============================================================================

// setupTestDB initializes a test database connection.
// Uses test database environment variables or defaults to localhost.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Try to connect to test database
	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("skipping: could not open test database: %v", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("skipping: could not connect to test database: %v", err)
	}

	// Run migrations if needed
	if err := runTestMigrations(t, db); err != nil {
		db.Close()
		t.Skipf("skipping: could not run migrations: %v", err)
	}

	return db
}

// cleanupTestDB removes test data and closes the database connection.
// Cleans up in reverse dependency order to respect foreign keys.
func cleanupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db == nil {
		return
	}

	// Clean up in reverse dependency order
	tables := []string{
		"gate_audit_log",
		"gate_recordings",
		"gate_sessions",
		"gate_ca_keys",
		"devices",
	}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Logf("warning: failed to clean table %s: %v", table, err)
		}
	}

	db.Close()
}

// runTestMigrations ensures all required tables exist for testing.
func runTestMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	// Create devices table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			id VARCHAR(36) PRIMARY KEY,
			device_name VARCHAR(255) NOT NULL,
			device_type VARCHAR(50) NOT NULL,
			state VARCHAR(50) NOT NULL,
			approved_by VARCHAR(255),
			approved_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create devices table: %w", err)
	}

	// Create gate_sessions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gate_sessions (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL,
			state VARCHAR(50) NOT NULL,
			cert_serial VARCHAR(255),
			cert_expires_at TIMESTAMP,
			started_at TIMESTAMP,
			connected_at TIMESTAMP,
			disconnected_at TIMESTAMP,
			ssh_port INT,
			recording_id VARCHAR(36),
			disconnect_reason VARCHAR(255),
			gui_protocol VARCHAR(50),
			x11_display_port INT,
			vnc_port INT,
			gui_session_started_at TIMESTAMP,
			gui_session_ended_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			FOREIGN KEY (device_id) REFERENCES devices(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create gate_sessions table: %w", err)
	}

	// Create gate_ca_keys table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gate_ca_keys (
			id VARCHAR(36) PRIMARY KEY,
			private_key_pem_encrypted TEXT NOT NULL,
			public_key_pem TEXT NOT NULL,
			expires_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create gate_ca_keys table: %w", err)
	}

	// Create gate_recordings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gate_recordings (
			id VARCHAR(36) PRIMARY KEY,
			session_id VARCHAR(36) NOT NULL,
			file_path TEXT NOT NULL,
			file_size BIGINT,
			duration_seconds INT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			FOREIGN KEY (session_id) REFERENCES gate_sessions(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create gate_recordings table: %w", err)
	}

	// Create gate_audit_log table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gate_audit_log (
			id VARCHAR(36) PRIMARY KEY,
			event_type VARCHAR(100) NOT NULL,
			actor VARCHAR(255) NOT NULL,
			details TEXT,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create gate_audit_log table: %w", err)
	}

	return nil
}
