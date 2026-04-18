package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
)

// setupRecordingTestDB creates a test database with recording schema.
func setupRecordingTestDB(t *testing.T) *sql.DB {
	// Use environment variables for test database connection
	// Default to localhost for local testing
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

	// Create tables if they don't exist
	schema := `
		CREATE TABLE IF NOT EXISTS devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			device_name TEXT NOT NULL,
			device_type TEXT NOT NULL,
			state TEXT NOT NULL,
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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS gate_recordings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_id UUID NOT NULL REFERENCES gate_sessions(id) ON DELETE CASCADE,
			user_id UUID NOT NULL,
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			file_path TEXT NOT NULL,
			timing_path TEXT,
			byte_size INTEGER,
			duration_seconds INTEGER,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err = db.Exec(schema)
	if err != nil {
		t.Skipf("skipping test: could not create test schema: %v", err)
	}

	// Clean up any existing test data
	db.Exec("DELETE FROM gate_recordings")
	db.Exec("DELETE FROM gate_sessions")
	db.Exec("DELETE FROM devices")

	return db
}

// cleanupRecordingTestDB removes test data.
func cleanupRecordingTestDB(t *testing.T, db *sql.DB) {
	// Clean up in reverse dependency order
	db.Exec("DELETE FROM gate_recordings")
	db.Exec("DELETE FROM gate_sessions")
	db.Exec("DELETE FROM devices")
	db.Close()
}

// Test 1: CreateRecording creates entry with correct paths
func TestCreateRecording(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440000"
	deviceID := createTestDevice(t, db)
	sessionID := createTestSession(t, db, userID, deviceID)

	recordingID, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording failed: %v", err)
	}

	if recordingID == "" {
		t.Fatal("expected non-empty recordingID")
	}

	// Verify in database
	rec, err := svc.GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording failed: %v", err)
	}

	if rec.ID != recordingID {
		t.Errorf("expected ID %s, got %s", recordingID, rec.ID)
	}
	if rec.SessionID != sessionID {
		t.Errorf("expected SessionID %s, got %s", sessionID, rec.SessionID)
	}
	if rec.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, rec.UserID)
	}
	if rec.DeviceID != deviceID {
		t.Errorf("expected DeviceID %s, got %s", deviceID, rec.DeviceID)
	}

	// Verify file path format: {recordingDir}/{sessionID}-{recordingID}.cast
	expectedPath := filepath.Join(tmpDir, sessionID+"-"+recordingID+".cast")
	if rec.FilePath != expectedPath {
		t.Errorf("expected FilePath %s, got %s", expectedPath, rec.FilePath)
	}

	// Verify timestamps
	if rec.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if rec.CompletedAt != nil {
		t.Error("expected nil CompletedAt for new recording")
	}
}

// Test 2: GetRecording retrieves recording by ID
func TestGetRecording(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440001"
	deviceID := createTestDevice(t, db)
	sessionID := createTestSession(t, db, userID, deviceID)

	recordingID, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording failed: %v", err)
	}

	// Retrieve recording
	rec, err := svc.GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording failed: %v", err)
	}

	if rec == nil {
		t.Fatal("expected non-nil recording")
	}
	if rec.ID != recordingID {
		t.Errorf("expected ID %s, got %s", recordingID, rec.ID)
	}

	// Test non-existent recording
	_, err = svc.GetRecording("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent recording")
	}
}

// Test 3: UpdateRecording sets byte_size and duration
func TestUpdateRecording(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440002"
	deviceID := createTestDevice(t, db)
	sessionID := createTestSession(t, db, userID, deviceID)

	recordingID, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording failed: %v", err)
	}

	// Update recording
	byteSize := int64(102400)
	durationSeconds := 3600

	err = svc.UpdateRecording(recordingID, byteSize, durationSeconds)
	if err != nil {
		t.Fatalf("UpdateRecording failed: %v", err)
	}

	// Verify update
	rec, err := svc.GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording failed: %v", err)
	}

	if rec.ByteSize == nil || *rec.ByteSize != byteSize {
		t.Errorf("expected ByteSize %d, got %v", byteSize, rec.ByteSize)
	}
	if rec.DurationSeconds == nil || *rec.DurationSeconds != durationSeconds {
		t.Errorf("expected DurationSeconds %d, got %v", durationSeconds, rec.DurationSeconds)
	}
	if rec.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt")
	}
}

// Test 4: DeleteRecording removes from DB and deletes file
func TestDeleteRecording(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440003"
	deviceID := createTestDevice(t, db)
	sessionID := createTestSession(t, db, userID, deviceID)

	recordingID, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording failed: %v", err)
	}

	// Get the file path
	rec, err := svc.GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording failed: %v", err)
	}
	filePath := rec.FilePath

	// Create the file so we can verify it gets deleted
	err = os.WriteFile(filePath, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("test file not created: %v", err)
	}

	// Delete recording
	err = svc.DeleteRecording(recordingID)
	if err != nil {
		t.Fatalf("DeleteRecording failed: %v", err)
	}

	// Verify removed from database
	_, err = svc.GetRecording(recordingID)
	if err == nil {
		t.Fatal("expected error for deleted recording")
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

// Test 5: ListRecordingsBySession returns correct recordings
func TestListRecordingsBySession(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440004"
	deviceID := createTestDevice(t, db)
	sessionID := createTestSession(t, db, userID, deviceID)

	// Create multiple recordings
	recordingID1, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording 1 failed: %v", err)
	}

	recordingID2, err := svc.CreateRecording(userID, deviceID, sessionID)
	if err != nil {
		t.Fatalf("CreateRecording 2 failed: %v", err)
	}

	// List recordings for session
	recordings, err := svc.ListRecordingsBySession(sessionID)
	if err != nil {
		t.Fatalf("ListRecordingsBySession failed: %v", err)
	}

	if len(recordings) != 2 {
		t.Errorf("expected 2 recordings, got %d", len(recordings))
	}

	// Verify recordings are in correct order (DESC by created_at)
	if recordings[0].ID != recordingID2 {
		t.Errorf("expected first recording to be %s, got %s", recordingID2, recordings[0].ID)
	}
	if recordings[1].ID != recordingID1 {
		t.Errorf("expected second recording to be %s, got %s", recordingID1, recordings[1].ID)
	}

	// Verify all recordings have correct session
	for _, rec := range recordings {
		if rec.SessionID != sessionID {
			t.Errorf("expected SessionID %s, got %s", sessionID, rec.SessionID)
		}
	}
}

// Test 6: ListRecordingsByDevice returns correct recordings
func TestListRecordingsByDevice(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer cleanupRecordingTestDB(t, db)

	tmpDir := t.TempDir()
	svc, err := NewRecordingService(db, tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	userID := "550e8400-e29b-41d4-a716-446655440005"
	deviceID := createTestDevice(t, db)
	sessionID1 := createTestSession(t, db, userID, deviceID)
	sessionID2 := createTestSession(t, db, userID, deviceID)

	// Create recordings for two sessions on same device
	recordingID1, err := svc.CreateRecording(userID, deviceID, sessionID1)
	if err != nil {
		t.Fatalf("CreateRecording 1 failed: %v", err)
	}

	recordingID2, err := svc.CreateRecording(userID, deviceID, sessionID2)
	if err != nil {
		t.Fatalf("CreateRecording 2 failed: %v", err)
	}

	// List recordings for device
	recordings, err := svc.ListRecordingsByDevice(deviceID)
	if err != nil {
		t.Fatalf("ListRecordingsByDevice failed: %v", err)
	}

	if len(recordings) != 2 {
		t.Errorf("expected 2 recordings, got %d", len(recordings))
	}

	// Verify all recordings have correct device
	for _, rec := range recordings {
		if rec.DeviceID != deviceID {
			t.Errorf("expected DeviceID %s, got %s", deviceID, rec.DeviceID)
		}
	}

	// Verify IDs are present
	foundIDs := make(map[string]bool)
	for _, rec := range recordings {
		foundIDs[rec.ID] = true
	}

	if !foundIDs[recordingID1] {
		t.Errorf("expected recording %s in results", recordingID1)
	}
	if !foundIDs[recordingID2] {
		t.Errorf("expected recording %s in results", recordingID2)
	}
}

// Test 7: NewRecordingService creates directory
func TestNewRecordingService(t *testing.T) {
	db := setupRecordingTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	recordingDir := filepath.Join(tmpDir, "recordings")

	// Directory should not exist yet
	if _, err := os.Stat(recordingDir); !os.IsNotExist(err) {
		t.Fatal("recording directory should not exist before service creation")
	}

	svc, err := NewRecordingService(db, recordingDir)
	if err != nil {
		t.Fatalf("NewRecordingService failed: %v", err)
	}

	if svc.db != db {
		t.Error("expected db to be set")
	}
	if svc.recordingDir != recordingDir {
		t.Errorf("expected recordingDir %s, got %s", recordingDir, svc.recordingDir)
	}

	// Verify directory was created
	if _, err := os.Stat(recordingDir); os.IsNotExist(err) {
		t.Fatal("recording directory was not created")
	}
}
