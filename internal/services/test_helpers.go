package services

import (
	"database/sql"
	"testing"
	"time"
)

// createTestDevice creates a test device for service tests.
func createTestDevice(t *testing.T, db *sql.DB) string {
	var deviceID string
	err := db.QueryRow(`
		INSERT INTO devices (device_name, device_type, state)
		VALUES ($1, $2, $3)
		RETURNING id
	`, "test-device", "edge", "APPROVED").Scan(&deviceID)

	if err != nil {
		t.Fatalf("failed to create test device: %v", err)
	}
	return deviceID
}

// createTestSession creates a test session for service tests.
func createTestSession(t *testing.T, db *sql.DB, userID, deviceID string) string {
	var sessionID string
	err := db.QueryRow(`
		INSERT INTO gate_sessions (user_id, device_id, state, started_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, userID, deviceID, "ACTIVE", time.Now()).Scan(&sessionID)

	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	return sessionID
}
