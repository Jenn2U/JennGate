package daemon

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jenn2U/JennGate/internal/services"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestNotifyGUISessionStart tests that NotifyGUISessionStart correctly updates
// the session with GUI protocol and port information.
func TestNotifyGUISessionStart(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupDaemonTestDB(t)
	defer db.Close()

	sessionService := services.NewSessionService(db)
	ds := NewDaemonServer(sessionService, nil, db)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a test session first
	session, err := sessionService.CreateSession(ctx, "user-123", "device-456", "cert-serial", time.Now().Add(1*time.Hour))
	require.NoError(t, err)

	// Notify daemon that GUI session started
	err = ds.NotifyGUISessionStart(ctx, session.ID, "vnc", 5900)
	require.NoError(t, err)

	// Verify session was updated with GUI status
	updated, err := sessionService.GetSession(ctx, session.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.GUIProtocol)
	require.Equal(t, "vnc", *updated.GUIProtocol)
	require.NotNil(t, updated.VNCPort)
	require.Equal(t, 5900, *updated.VNCPort)
	require.NotNil(t, updated.GUISessionStartedAt)
}

// TestNotifyGUISessionEnd tests that NotifyGUISessionEnd correctly clears
// GUI session data.
func TestNotifyGUISessionEnd(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupDaemonTestDB(t)
	defer db.Close()

	sessionService := services.NewSessionService(db)
	ds := NewDaemonServer(sessionService, nil, db)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a test session
	session, err := sessionService.CreateSession(ctx, "user-123", "device-456", "cert-serial", time.Now().Add(1*time.Hour))
	require.NoError(t, err)

	// Set up GUI session
	err = sessionService.UpdateSessionGUIStatus(ctx, session.ID, "x11", 6010, 0)
	require.NoError(t, err)

	// Notify daemon that GUI session ended
	err = ds.NotifyGUISessionEnd(ctx, session.ID)
	require.NoError(t, err)

	// Verify session was cleared
	updated, err := sessionService.GetSession(ctx, session.ID)
	require.NoError(t, err)
	require.Nil(t, updated.GUIProtocol)
	require.Nil(t, updated.VNCPort)
	require.Nil(t, updated.X11DisplayPort)
	require.NotNil(t, updated.GUISessionEndedAt)
}

// TestNotifyGUISessionStartWithX11 tests that NotifyGUISessionStart correctly
// handles X11 protocol.
func TestNotifyGUISessionStartWithX11(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupDaemonTestDB(t)
	defer db.Close()

	sessionService := services.NewSessionService(db)
	ds := NewDaemonServer(sessionService, nil, db)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a test session
	session, err := sessionService.CreateSession(ctx, "user-123", "device-456", "cert-serial", time.Now().Add(1*time.Hour))
	require.NoError(t, err)

	// Notify daemon that X11 session started
	err = ds.NotifyGUISessionStart(ctx, session.ID, "x11", 6010)
	require.NoError(t, err)

	// Verify session was updated with X11 information
	updated, err := sessionService.GetSession(ctx, session.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.GUIProtocol)
	require.Equal(t, "x11", *updated.GUIProtocol)
	require.NotNil(t, updated.X11DisplayPort)
	require.Equal(t, 6010, *updated.X11DisplayPort)
}

// ===================================================================
// Test Helper Functions
// ===================================================================

// hasTestDB checks if the test database is available.
func hasTestDB() bool {
	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

// setupDaemonTestDB creates a PostgreSQL database connection for daemon server testing.
func setupDaemonTestDB(t *testing.T) *sql.DB {
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

	// Create the devices and gate_sessions tables if they don't exist
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		device_name TEXT NOT NULL,
		device_type TEXT NOT NULL,
		state TEXT NOT NULL,
		approved_at TIMESTAMP,
		decommissioned_at TIMESTAMP,
		decommissioned_by TEXT,
		daemon_version TEXT,
		public_key_pem TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS gate_sessions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		state TEXT NOT NULL,
		cert_serial TEXT,
		cert_expires_at TIMESTAMP,
		started_at TIMESTAMP NOT NULL,
		connected_at TIMESTAMP,
		disconnected_at TIMESTAMP,
		ssh_port INTEGER DEFAULT 2222,
		recording_id UUID,
		disconnect_reason TEXT,
		gui_protocol TEXT,
		x11_display_port INTEGER,
		vnc_port INTEGER,
		gui_session_started_at TIMESTAMP,
		gui_session_ended_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_gate_sessions_user_device ON gate_sessions(user_id, device_id);
	CREATE INDEX IF NOT EXISTS idx_gate_sessions_state ON gate_sessions(state);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Skipf("skipping test: could not create schema: %v", err)
	}

	// Insert a test device
	insertTestDevice(t, db)

	// Clean up any existing test data
	db.Exec("DELETE FROM gate_sessions;")

	return db
}

// insertTestDevice inserts a test device into the devices table.
func insertTestDevice(t *testing.T, db *sql.DB) {
	query := `
	INSERT INTO devices (device_name, device_type, state, public_key_pem)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT DO NOTHING
	`

	_, err := db.Exec(query, "test-device", "edge-node", "APPROVED", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIF...")
	if err != nil {
		t.Skipf("skipping test: could not insert test device: %v", err)
	}
}
