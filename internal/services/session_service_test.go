package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

// TestSessionServiceCreateSessionCreatesNewSessionInRequestedState tests that CreateSession
// creates a new session in REQUESTED state with all required fields.
func TestSessionServiceCreateSessionCreatesNewSessionInRequestedState(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	userID := "user-123"
	deviceID := "device-456"
	certSerial := "cert-serial-789"
	certExpiresAt := time.Now().Add(1 * time.Hour)

	session, err := service.CreateSession(ctx, userID, deviceID, certSerial, certExpiresAt)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("CreateSession returned nil session")
	}

	if session.ID == "" {
		t.Fatal("session ID is empty")
	}

	if session.State != "REQUESTED" {
		t.Errorf("expected state REQUESTED, got %s", session.State)
	}

	if session.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, session.UserID)
	}

	if session.DeviceID != deviceID {
		t.Errorf("expected deviceID %s, got %s", deviceID, session.DeviceID)
	}

	if session.CertSerial != certSerial {
		t.Errorf("expected certSerial %s, got %s", certSerial, session.CertSerial)
	}

	if session.SSHPort != 2222 {
		t.Errorf("expected SSHPort 2222, got %d", session.SSHPort)
	}

	if session.ConnectedAt != nil {
		t.Error("expected ConnectedAt to be nil for new session")
	}

	if session.DisconnectedAt != nil {
		t.Error("expected DisconnectedAt to be nil for new session")
	}

	if !session.StartedAt.Before(time.Now().Add(1 * time.Second)) {
		t.Error("StartedAt should be approximately now")
	}
}

// TestSessionServiceStateTransitionsHappyPath tests valid state transitions:
// REQUESTED -> AUTHORIZED -> ACTIVE -> DISCONNECTED
func TestSessionServiceStateTransitionsHappyPath(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Create session
	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessionID := session.ID

	// Verify initial state
	session, err = service.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session.State != "REQUESTED" {
		t.Errorf("expected initial state REQUESTED, got %s", session.State)
	}

	// Transition to AUTHORIZED
	err = service.UpdateSessionState(ctx, sessionID, "AUTHORIZED")
	if err != nil {
		t.Fatalf("UpdateSessionState to AUTHORIZED failed: %v", err)
	}

	session, err = service.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession after AUTHORIZED transition failed: %v", err)
	}
	if session.State != "AUTHORIZED" {
		t.Errorf("expected state AUTHORIZED, got %s", session.State)
	}

	// Transition to ACTIVE via MarkConnected
	err = service.MarkConnected(ctx, sessionID)
	if err != nil {
		t.Fatalf("MarkConnected failed: %v", err)
	}

	session, err = service.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession after MarkConnected failed: %v", err)
	}
	if session.State != "ACTIVE" {
		t.Errorf("expected state ACTIVE, got %s", session.State)
	}

	if session.ConnectedAt == nil {
		t.Error("expected ConnectedAt to be set after MarkConnected")
	}

	// Transition to DISCONNECTED
	err = service.DisconnectSession(ctx, sessionID, "user_logout")
	if err != nil {
		t.Fatalf("DisconnectSession failed: %v", err)
	}

	session, err = service.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession after DisconnectSession failed: %v", err)
	}
	if session.State != "DISCONNECTED" {
		t.Errorf("expected state DISCONNECTED, got %s", session.State)
	}

	if session.DisconnectedAt == nil {
		t.Error("expected DisconnectedAt to be set")
	}

	if session.DisconnectReason == nil || *session.DisconnectReason != "user_logout" {
		t.Error("expected DisconnectReason to be 'user_logout'")
	}
}

// TestSessionServiceInvalidStateTransitionRejected tests that invalid state transitions
// are rejected with an error.
func TestSessionServiceInvalidStateTransitionRejected(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Create session
	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessionID := session.ID

	// Try invalid transition: REQUESTED -> ACTIVE (should fail)
	err = service.UpdateSessionState(ctx, sessionID, "ACTIVE")
	if err == nil {
		t.Fatal("expected error for invalid transition REQUESTED -> ACTIVE")
	}

	// Verify state didn't change
	session, err = service.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session.State != "REQUESTED" {
		t.Errorf("state should not have changed, expected REQUESTED, got %s", session.State)
	}

	// Try invalid state value
	err = service.UpdateSessionState(ctx, sessionID, "INVALID_STATE")
	if err == nil {
		t.Fatal("expected error for invalid state value")
	}
}

// TestSessionServiceGetSessionRetrievesSessionCorrectly tests that GetSession
// correctly retrieves session data from the database.
func TestSessionServiceGetSessionRetrievesSessionCorrectly(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	userID := "user-999"
	deviceID := "device-888"
	certSerial := "serial-777"
	certExpiresAt := time.Now().Add(2 * time.Hour)

	// Create session
	createdSession, err := service.CreateSession(ctx, userID, deviceID, certSerial, certExpiresAt)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Retrieve session
	retrievedSession, err := service.GetSession(ctx, createdSession.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrievedSession.ID != createdSession.ID {
		t.Errorf("ID mismatch: expected %s, got %s", createdSession.ID, retrievedSession.ID)
	}

	if retrievedSession.UserID != userID {
		t.Errorf("UserID mismatch: expected %s, got %s", userID, retrievedSession.UserID)
	}

	if retrievedSession.DeviceID != deviceID {
		t.Errorf("DeviceID mismatch: expected %s, got %s", deviceID, retrievedSession.DeviceID)
	}

	if retrievedSession.State != "REQUESTED" {
		t.Errorf("State mismatch: expected REQUESTED, got %s", retrievedSession.State)
	}

	// Test non-existent session
	_, err = service.GetSession(ctx, "non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// TestSessionServiceListActiveSessionsReturnsOnlyActiveSessions tests that
// ListActiveSessions returns only ACTIVE sessions for a device.
func TestSessionServiceListActiveSessionsReturnsOnlyActiveSessions(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	deviceID := "test-device-001"

	// Create multiple sessions
	session1, err := service.CreateSession(ctx, "user-1", deviceID, "cert-1", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 1 failed: %v", err)
	}

	session2, err := service.CreateSession(ctx, "user-2", deviceID, "cert-2", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 2 failed: %v", err)
	}

	_, err = service.CreateSession(ctx, "user-3", deviceID, "cert-3", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 3 failed: %v", err)
	}

	// Transition session1 to ACTIVE
	service.UpdateSessionState(ctx, session1.ID, "AUTHORIZED")
	service.MarkConnected(ctx, session1.ID)

	// Transition session2 to AUTHORIZED (not ACTIVE)
	service.UpdateSessionState(ctx, session2.ID, "AUTHORIZED")

	// Leave session3 in REQUESTED state

	// List active sessions
	activeSessions, err := service.ListActiveSessions(ctx, deviceID)
	if err != nil {
		t.Fatalf("ListActiveSessions failed: %v", err)
	}

	if len(activeSessions) != 1 {
		t.Errorf("expected 1 active session, got %d", len(activeSessions))
	}

	if activeSessions[0].ID != session1.ID {
		t.Errorf("expected active session ID %s, got %s", session1.ID, activeSessions[0].ID)
	}

	if activeSessions[0].State != "ACTIVE" {
		t.Errorf("expected state ACTIVE, got %s", activeSessions[0].State)
	}
}

// TestSessionServiceListSessionsByDeviceReturnsAllSessions tests that
// ListSessionsByDevice returns all sessions for a device, ordered by creation time.
func TestSessionServiceListSessionsByDeviceReturnsAllSessions(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	deviceID := "test-device-002"

	// Create sessions
	session1, err := service.CreateSession(ctx, "user-1", deviceID, "cert-1", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 1 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	session2, err := service.CreateSession(ctx, "user-2", deviceID, "cert-2", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 2 failed: %v", err)
	}

	// List all sessions for device
	sessions, err := service.ListSessionsByDevice(ctx, deviceID)
	if err != nil {
		t.Fatalf("ListSessionsByDevice failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Verify order (most recent first)
	if sessions[0].ID != session2.ID || sessions[1].ID != session1.ID {
		t.Error("sessions not ordered by creation time (most recent first)")
	}
}

// TestSessionServiceListSessionsByUser tests that ListSessionsByUser returns
// all sessions for a specific user.
func TestSessionServiceListSessionsByUser(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Setup: insert test device
	deviceID := "device-123"
	user1 := "user-1"
	user2 := "user-2"
	insertTestDevice(t, db)

	// Create sessions for two different users
	sess1, err := service.CreateSession(ctx, user1, deviceID, "cert-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 1 failed: %v", err)
	}

	sess2, err := service.CreateSession(ctx, user1, deviceID, "cert-2", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 2 failed: %v", err)
	}

	sess3, err := service.CreateSession(ctx, user2, deviceID, "cert-3", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession 3 failed: %v", err)
	}

	// List sessions for user1
	sessions, err := service.ListSessionsByUser(ctx, user1)
	if err != nil {
		t.Fatalf("ListSessionsByUser failed: %v", err)
	}

	// Should return 2 sessions for user1 only
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for user1, got %d", len(sessions))
	}

	// Verify we got user1's sessions (most recent first)
	if sessions[0].ID != sess2.ID || sessions[1].ID != sess1.ID {
		t.Errorf("returned sessions don't match expected IDs or order")
	}

	// Verify user2's session is not included
	for _, s := range sessions {
		if s.UserID != user1 {
			t.Errorf("found session for different user: expected %s, got %s", user1, s.UserID)
		}
	}

	// List sessions for user2
	sessions, err = service.ListSessionsByUser(ctx, user2)
	if err != nil {
		t.Fatalf("ListSessionsByUser for user2 failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session for user2, got %d", len(sessions))
	}

	if sessions[0].ID != sess3.ID {
		t.Errorf("expected session ID %s, got %s", sess3.ID, sessions[0].ID)
	}
}

// TestSessionServiceMarkConnectedFailsIfNotAuthorized tests that MarkConnected
// only works when session is in AUTHORIZED state.
func TestSessionServiceMarkConnectedFailsIfNotAuthorized(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Try to mark connected while in REQUESTED state (should fail)
	err = service.MarkConnected(ctx, session.ID)
	if err == nil {
		t.Fatal("expected error when marking connected from REQUESTED state")
	}

	// Verify state is still REQUESTED
	session, err = service.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session.State != "REQUESTED" {
		t.Errorf("expected state REQUESTED, got %s", session.State)
	}
}

// TestSessionServiceDisconnectSessionSetsReason tests that DisconnectSession
// correctly sets the disconnect reason.
func TestSessionServiceDisconnectSessionSetsReason(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Transition to ACTIVE
	service.UpdateSessionState(ctx, session.ID, "AUTHORIZED")
	service.MarkConnected(ctx, session.ID)

	// Disconnect with specific reason
	reason := "timeout"
	err = service.DisconnectSession(ctx, session.ID, reason)
	if err != nil {
		t.Fatalf("DisconnectSession failed: %v", err)
	}

	// Verify reason was set
	session, err = service.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session.DisconnectReason == nil || *session.DisconnectReason != reason {
		t.Errorf("expected disconnect reason %s, got %v", reason, session.DisconnectReason)
	}
}

// TestSessionServiceCannotTransitionFromDisconnected tests that we cannot
// transition a session that is already disconnected.
func TestSessionServiceCannotTransitionFromDisconnected(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Transition to DISCONNECTED
	service.UpdateSessionState(ctx, session.ID, "DISCONNECTED")

	// Try to transition from DISCONNECTED (should fail)
	err = service.DisconnectSession(ctx, session.ID, "already_disconnected")
	if err == nil {
		t.Fatal("expected error when disconnecting already disconnected session")
	}
}

// TestSessionServiceCleanupExpiredSessions tests that CleanupExpiredSessions
// marks sessions with expired certificates as DISCONNECTED.
func TestSessionServiceCleanupExpiredSessions(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Create session with expired certificate
	expiredTime := time.Now().Add(-1 * time.Hour)
	session, err := service.CreateSession(
		ctx,
		"user-123",
		"device-456",
		"cert-serial",
		expiredTime,
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Transition to AUTHORIZED (active state)
	service.UpdateSessionState(ctx, session.ID, "AUTHORIZED")

	// Run cleanup
	err = service.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions failed: %v", err)
	}

	// Verify session was disconnected
	session, err = service.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session.State != "DISCONNECTED" {
		t.Errorf("expected state DISCONNECTED after cleanup, got %s", session.State)
	}

	if session.DisconnectReason == nil || *session.DisconnectReason != "cert_expired" {
		t.Error("expected disconnect reason to be 'cert_expired'")
	}
}

// TestUpdateSessionGUIStatus tests that UpdateSessionGUIStatus correctly updates
// the session with GUI protocol and port information.
func TestUpdateSessionGUIStatus(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Create a test session first
	session, err := service.CreateSession(ctx, "user-123", "device-456", "cert-serial", time.Now().Add(1*time.Hour))
	require.NoError(t, err)

	// Update GUI status
	err = service.UpdateSessionGUIStatus(ctx, session.ID, "vnc", 5900, 0)
	require.NoError(t, err)

	// Verify update
	updated, err := service.GetSession(ctx, session.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.GUIProtocol)
	require.Equal(t, "vnc", *updated.GUIProtocol)
	require.NotNil(t, updated.VNCPort)
	require.Equal(t, 5900, *updated.VNCPort)
	require.NotNil(t, updated.GUISessionStartedAt)
}

// TestEndGUISession tests that EndGUISession correctly clears GUI session data
// and records the end time.
func TestEndGUISession(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupSessionTestDB(t)
	defer db.Close()

	service := NewSessionService(db)
	ctx := context.Background()

	// Create session with active GUI
	session, err := service.CreateSession(ctx, "user-123", "device-456", "cert-serial", time.Now().Add(1*time.Hour))
	require.NoError(t, err)

	// Set up GUI session
	err = service.UpdateSessionGUIStatus(ctx, session.ID, "x11", 6010, 0)
	require.NoError(t, err)

	// End GUI session
	err = service.EndGUISession(ctx, session.ID)
	require.NoError(t, err)

	// Verify cleanup
	updated, err := service.GetSession(ctx, session.ID)
	require.NoError(t, err)
	require.Nil(t, updated.GUIProtocol)
	require.Nil(t, updated.VNCPort)
	require.Nil(t, updated.X11DisplayPort)
	require.NotNil(t, updated.GUISessionEndedAt)
}

// Helper functions

// setupSessionTestDB creates a PostgreSQL database connection for session service testing.
func setupSessionTestDB(t *testing.T) *sql.DB {
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
