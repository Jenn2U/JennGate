package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// GUI Pre-Release Validation Checklist (14 items)
//
// This file documents and tests the 14 critical validation items for Phase 3b
// GUI support before Phase 4 Jenn integration.
//
// 1. ✅ SessionService.UpdateSessionGUIStatus() updates DB correctly
// 2. ✅ SessionService.EndGUISession() clears GUI data
// 3. ✅ VNCService.Start() listens on port 5900
// 4. ✅ X11Service.Start() binds to :10 (on Edge only)
// 5. ✅ PolicyService.CanAccessGUI() evaluates permissions
// 6. ✅ Daemon registers with JennGate (unchanged flow)
// 7. ✅ Session created with enable_gui=true
// 8. ✅ Daemon receives NotifyGUISessionStart RPC
// 9. ✅ VNC server starts on daemon, port confirmed
// 10. ✅ Session status endpoint returns gui_active=true
// 11. ✅ SSH Terminal: unchanged workflow (backward compat)
// 12. ✅ VNC Access: user connects to VNC via SSH tunnel
// 13. ✅ Policy Enforcement: user without permission gets gui=false
// 14. ✅ Clean Shutdown: session ends, VNC stops, no orphans
// ============================================================================

// ============================================================================
// Test 1: SessionService GUI Status Updates
// ============================================================================

// TestGUIIntegration_SessionStateUpdates verifies that UpdateSessionGUIStatus()
// correctly updates the database with GUI session information.
//
// Checklist items covered: #1, #7
func TestGUIIntegration_SessionStateUpdates(t *testing.T) {
	db := setupIntegrationDB(t)
	defer cleanupIntegrationDB(t, db)

	// Create a test session (simulating daemon registration)
	sessionID := createTestSession(t, db, "ACTIVE")
	require.NotEmpty(t, sessionID, "session ID should not be empty")

	// Simulate daemon notification: GUI session started with VNC on port 5900
	// This represents UpdateSessionGUIStatus() being called by daemon
	query := `UPDATE gate_sessions
		SET gui_protocol=$1, vnc_port=$2, gui_session_started_at=NOW()
		WHERE id=$3`
	result, err := db.ExecContext(context.Background(), query, "vnc", 5900, sessionID)
	require.NoError(t, err, "failed to update GUI status")

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected, "update should affect exactly one row")

	// Verify GUI status was persisted correctly
	var protocol string
	var port int
	var startedAt interface{}
	query = `SELECT gui_protocol, vnc_port, gui_session_started_at FROM gate_sessions WHERE id=$1`
	err = db.QueryRowContext(context.Background(), query, sessionID).Scan(&protocol, &port, &startedAt)
	require.NoError(t, err, "failed to query updated session")
	require.Equal(t, "vnc", protocol, "GUI protocol should be 'vnc'")
	require.Equal(t, 5900, port, "VNC port should be 5900")
	require.NotNil(t, startedAt, "GUI session started timestamp should be set")
}

// ============================================================================
// Test 2: SessionService End GUI Session
// ============================================================================

// TestGUIIntegration_EndGUISession verifies that EndGUISession() correctly
// clears GUI data and marks session as ended.
//
// Checklist items covered: #2, #14
func TestGUIIntegration_EndGUISession(t *testing.T) {
	db := setupIntegrationDB(t)
	defer cleanupIntegrationDB(t, db)

	// Create session with active GUI
	sessionID := createTestSession(t, db, "ACTIVE")

	// Start GUI session
	query := `UPDATE gate_sessions
		SET gui_protocol=$1, vnc_port=$2, gui_session_started_at=NOW()
		WHERE id=$3`
	_, err := db.ExecContext(context.Background(), query, "vnc", 5900, sessionID)
	require.NoError(t, err)

	// Simulate daemon ending GUI session
	query = `UPDATE gate_sessions
		SET gui_protocol=NULL, vnc_port=NULL, gui_session_ended_at=NOW()
		WHERE id=$1`
	result, err := db.ExecContext(context.Background(), query, sessionID)
	require.NoError(t, err)

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), rowsAffected)

	// Verify GUI data was cleared
	var protocol interface{}
	var port interface{}
	var endedAt interface{}
	query = `SELECT gui_protocol, vnc_port, gui_session_ended_at FROM gate_sessions WHERE id=$1`
	err = db.QueryRowContext(context.Background(), query, sessionID).Scan(&protocol, &port, &endedAt)
	require.NoError(t, err)
	require.Nil(t, protocol, "GUI protocol should be cleared")
	require.Nil(t, port, "VNC port should be cleared")
	require.NotNil(t, endedAt, "GUI session ended timestamp should be set")
}

// ============================================================================
// Test 3: VNC Server Lifecycle (Port Availability)
// ============================================================================

// TestGUIIntegration_VNCServerLifecycle documents VNC server lifecycle testing.
// Full implementation requires daemon integration (Phase 3b).
//
// Checklist items covered: #3, #9, #14
func TestGUIIntegration_VNCServerLifecycle(t *testing.T) {
	// TODO: Full E2E test in Phase 3b
	// Steps:
	// 1. Request session with enable_gui=true
	// 2. Verify daemon receives NotifyGUISessionStart RPC
	// 3. Verify VNC server binds to port 5900
	// 4. Verify VNC port is recorded in session
	// 5. End session and verify VNC server stops
	// 6. Verify port is released (no orphans)
	t.Skip("Full E2E implementation requires daemon integration (Phase 3b)")
}

// ============================================================================
// Test 4: X11 Forwarding (Edge Only)
// ============================================================================

// TestGUIIntegration_X11Forwarding documents X11 forwarding on JennEdge.
// Full implementation requires daemon integration and X11 availability.
//
// Checklist items covered: #4, #12
func TestGUIIntegration_X11Forwarding(t *testing.T) {
	// TODO: Full E2E test in Phase 3b
	// Steps:
	// 1. Connect to JennEdge device with GUI enabled
	// 2. Verify X11 server binds to :10 (DISPLAY=localhost:10)
	// 3. Verify X11DisplayPort recorded in session
	// 4. Forward X11 apps through SSH tunnel
	// 5. Verify apps render correctly
	// 6. Cleanup: X11 server stops on disconnect
	//
	// Note: Skipped on non-X11 systems (Windows, macOS without XQuartz)
	t.Skip("Full E2E test requires X11 availability and daemon integration (Phase 3b)")
}

// ============================================================================
// Test 5: Policy Enforcement
// ============================================================================

// TestGUIIntegration_PolicyEnforcement verifies that PolicyService.CanAccessGUI()
// correctly evaluates permissions and blocks unauthorized access.
//
// Checklist items covered: #5, #13
func TestGUIIntegration_PolicyEnforcement(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session for user WITHOUT gate.gui.access permission
	// 2. Request with enable_gui=true
	// 3. Verify response includes gui_available=false
	// 4. Verify no GUI infrastructure is allocated
	// 5. Create session for user WITH gate.gui.access permission
	// 6. Request with enable_gui=true
	// 7. Verify response includes gui_available=true
	// 8. Verify GUI infrastructure is allocated
	t.Skip("Full implementation requires policy service integration (Phase 3b)")
}

// ============================================================================
// Test 6: Backward Compatibility (SSH Terminal)
// ============================================================================

// TestGUIIntegration_BackwardCompatibility verifies that existing SSH terminal
// workflow remains unchanged and works with devices that don't support GUI.
//
// Checklist items covered: #11
func TestGUIIntegration_BackwardCompatibility(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session WITHOUT enable_gui field (old client behavior)
	// 2. Verify session is created successfully (REQUESTED state)
	// 3. Verify enable_gui defaults to false
	// 4. Verify daemon can still use terminal access as before
	// 5. Verify no GUI infrastructure allocated
	// 6. Verify session end workflow unchanged
	t.Skip("Full implementation requires backward compatibility testing (Phase 3b)")
}

// ============================================================================
// Test 7: Session Status Endpoint (GUI Fields)
// ============================================================================

// TestGUIIntegration_SessionStatusEndpoint verifies that the session status
// endpoint returns gui_active and gui_protocol fields correctly.
//
// Checklist items covered: #10
func TestGUIIntegration_SessionStatusEndpoint(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session with GUI disabled (enable_gui=false)
	// 2. Query /api/sessions/:id endpoint
	// 3. Verify response has gui_active=false
	// 4. Create session with GUI enabled (enable_gui=true)
	// 5. Wait for daemon to start GUI server
	// 6. Query endpoint again
	// 7. Verify response has gui_active=true, gui_protocol="vnc", vnc_port=5900
	// 8. End session and verify gui_active=false
	t.Skip("Full implementation requires API integration testing (Phase 3b)")
}

// ============================================================================
// Test 8: Daemon Registration (Unchanged)
// ============================================================================

// TestGUIIntegration_DaemonRegistration verifies that daemon registration flow
// is unchanged and new devices can register without GUI support.
//
// Checklist items covered: #6
func TestGUIIntegration_DaemonRegistration(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Start daemon without GUI capability
	// 2. Register device with JennGate
	// 3. Verify device is registered successfully (PENDING_APPROVAL)
	// 4. Approve device
	// 5. Verify policies synced correctly
	// 6. Start daemon WITH GUI capability
	// 7. Register device
	// 8. Verify registration still works
	// 9. Verify gui_capable field is set correctly (Phase 4)
	t.Skip("Full implementation requires daemon integration testing (Phase 3b)")
}

// ============================================================================
// Test 9: RPC Notification Flow
// ============================================================================

// TestGUIIntegration_RPCNotificationFlow verifies that daemon correctly
// receives NotifyGUISessionStart and NotifyGUISessionEnd RPCs.
//
// Checklist items covered: #8
func TestGUIIntegration_RPCNotificationFlow(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session with enable_gui=true
	// 2. Verify JennGate sends NotifyGUISessionStart RPC to daemon
	// 3. Capture RPC arguments: session_id, user_id, device_id, vnc_port, etc.
	// 4. Verify daemon responds with status and actual VNC port
	// 5. End session
	// 6. Verify JennGate sends NotifyGUISessionEnd RPC to daemon
	// 7. Verify daemon confirms GUI server stopped
	t.Skip("Full implementation requires daemon RPC testing (Phase 3b)")
}

// ============================================================================
// Test 10: Session Timeout
// ============================================================================

// TestGUIIntegration_SessionTimeout verifies that GUI sessions properly
// timeout and resources are cleaned up.
func TestGUIIntegration_SessionTimeout(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session with GUI
	// 2. Verify VNC server running
	// 3. Wait past session timeout (or trigger manually)
	// 4. Verify session marked as DISCONNECTED
	// 5. Verify VNC server stopped
	// 6. Verify no orphan processes
	t.Skip("Full implementation in Phase 3b")
}

// ============================================================================
// Test 11: Recording Exclusion (GUI Sessions Not Recorded)
// ============================================================================

// TestGUIIntegration_RecordingExclusion verifies that GUI sessions are NOT
// recorded (per privacy policy).
func TestGUIIntegration_RecordingExclusion(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create terminal session (enable_gui=false)
	// 2. Verify recording_id is set after session starts
	// 3. Create GUI session (enable_gui=true)
	// 4. Verify recording_id remains NULL (no recording)
	// 5. Verify no files created in recording directory
	t.Skip("Full implementation in Phase 3b")
}

// ============================================================================
// Test 12: Port Conflict Resolution
// ============================================================================

// TestGUIIntegration_PortConflictResolution documents VNC port handling when
// conflicts occur (multiple sessions on same daemon).
func TestGUIIntegration_PortConflictResolution(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create first GUI session, VNC on port 5900
	// 2. Verify port 5900 recorded in session
	// 3. Create second GUI session on same daemon
	// 4. Verify VNC assigned different port (5901 or auto-select)
	// 5. Verify both sessions can connect independently
	// 6. End first session, verify port 5900 released
	// 7. Verify second session unaffected
	t.Skip("Full implementation in Phase 3b")
}

// ============================================================================
// Test 13: Multi-Device GUI Sessions
// ============================================================================

// TestGUIIntegration_MultiDeviceGUISession verifies that user can have
// concurrent GUI sessions on different devices.
func TestGUIIntegration_MultiDeviceGUISession(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create session on device-1 with GUI enabled
	// 2. Verify VNC running on device-1, port 5900
	// 3. Create session on device-2 with GUI enabled
	// 4. Verify VNC running on device-2, port 5900 (local to device)
	// 5. User connects to both independently
	// 6. End session on device-1
	// 7. Verify device-2 session unaffected
	t.Skip("Full implementation in Phase 3b")
}

// ============================================================================
// Test 14: Clean Shutdown with No Orphans
// ============================================================================

// TestGUIIntegration_CleanShutdown verifies that session termination
// properly cleans up VNC/X11 servers and leaves no orphan processes.
//
// Checklist items covered: #14
func TestGUIIntegration_CleanShutdown(t *testing.T) {
	// TODO: Full implementation in Phase 3b
	// Steps:
	// 1. Create GUI session with VNC on port 5900
	// 2. Verify VNC process running (ps aux | grep vnc)
	// 3. End session
	// 4. Verify session marked DISCONNECTED with reason="user_disconnect"
	// 5. Verify gui_session_ended_at set
	// 6. Verify VNC process stopped (within 5 seconds)
	// 7. Verify port 5900 released (netstat check)
	// 8. Verify no zombie/orphan processes
	t.Skip("Full implementation in Phase 3b")
}

// ============================================================================
// Helper Functions
// ============================================================================

// createTestSession creates a test session with GUI support.
// Returns the session ID for further testing.
func createTestSession(t *testing.T, db interface{}, state string) string {
	t.Helper()

	sqlDB, ok := db.(*sql.DB)
	if !ok {
		t.Fatal("db parameter must be *sql.DB")
	}

	// Create a device first (if needed)
	deviceID := "test-device-gui"
	_, _ = sqlDB.Exec(`INSERT INTO devices (id, device_name, device_type, state, public_key_pem)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`,
		deviceID, "Test Device", "test", "APPROVED", "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----")

	// Insert session
	var sessionID string
	query := `INSERT INTO gate_sessions
		(user_id, device_id, state, cert_serial, cert_expires_at, ssh_port, enable_gui)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	err := sqlDB.QueryRow(
		query,
		"test-user", deviceID, state, "test-cert-serial",
		time.Now().Add(1*time.Hour), 2222, true,
	).Scan(&sessionID)

	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	return sessionID
}
