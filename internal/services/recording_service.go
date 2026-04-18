package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RecordingService manages session recording metadata and file lifecycle.
// It tracks recording transcripts and handles cleanup on decommission.
type RecordingService struct {
	db           *sql.DB
	recordingDir string
	mu           sync.RWMutex
}

// Recording represents a session recording with metadata.
// Stores session transcript paths and duration information.
type Recording struct {
	ID              string
	SessionID       string
	UserID          string
	DeviceID        string
	FilePath        string
	TimingPath      *string
	ByteSize        *int64
	DurationSeconds *int
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

// NewRecordingService creates a new RecordingService with database connection.
// It creates the recordingDir if it doesn't exist and validates DB connectivity.
func NewRecordingService(db *sql.DB, recordingDir string) (*RecordingService, error) {
	if recordingDir == "" {
		recordingDir = "/var/lib/jenn-edge/recordings"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(recordingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create recording directory: %w", err)
	}

	// Validate database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate database connection: %w", err)
	}

	return &RecordingService{
		db:           db,
		recordingDir: recordingDir,
	}, nil
}

// CreateRecording creates a new recording entry for a session.
// It generates a UUID for the recording and formats the file path as
// {recordingDir}/{sessionID}-{recordingID}.cast (asciinema format).
func (r *RecordingService) CreateRecording(ctx context.Context, userID, deviceID, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordingID := uuid.New().String()
	now := time.Now()

	// Format file path: {recordingDir}/{sessionID}-{recordingID}.cast
	filePath := filepath.Join(r.recordingDir, fmt.Sprintf("%s-%s.cast", sessionID, recordingID))

	query := `
		INSERT INTO gate_recordings (id, session_id, user_id, device_id, file_path, started_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		recordingID, sessionID, userID, deviceID, filePath, now, now,
	)

	if err != nil {
		return "", fmt.Errorf("failed to create recording: %w", err)
	}

	return recordingID, nil
}

// GetRecording retrieves a recording by ID.
// Returns the Recording struct with all metadata or an error if not found.
func (r *RecordingService) GetRecording(ctx context.Context, recordingID string) (*Recording, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, session_id, user_id, device_id, file_path, timing_path, byte_size, duration_seconds, started_at, completed_at, created_at
		FROM gate_recordings
		WHERE id = $1
	`

	rec := &Recording{}
	err := r.db.QueryRowContext(ctx, query, recordingID).Scan(
		&rec.ID, &rec.SessionID, &rec.UserID, &rec.DeviceID, &rec.FilePath,
		&rec.TimingPath, &rec.ByteSize, &rec.DurationSeconds, &rec.StartedAt,
		&rec.CompletedAt, &rec.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recording not found: %s", recordingID)
		}
		return nil, fmt.Errorf("failed to get recording: %w", err)
	}

	return rec, nil
}

// UpdateRecording updates the recording metadata after the session ends.
// Sets byte_size, duration_seconds, and completed_at timestamp.
func (r *RecordingService) UpdateRecording(ctx context.Context, recordingID string, byteSize int64, durationSeconds int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	query := `
		UPDATE gate_recordings
		SET byte_size = $1, duration_seconds = $2, completed_at = $3
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query, byteSize, durationSeconds, now, recordingID)
	if err != nil {
		return fmt.Errorf("failed to update recording: %w", err)
	}

	return nil
}

// DeleteRecording deletes a recording from the database and removes the physical file.
// Errors from file deletion are ignored (file may already be gone).
// Used on device decommission or session cleanup.
func (r *RecordingService) DeleteRecording(ctx context.Context, recordingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get the file path before deleting from DB
	var filePath string
	err := r.db.QueryRowContext(ctx, "SELECT file_path FROM gate_recordings WHERE id = $1", recordingID).Scan(&filePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("recording not found: %s", recordingID)
		}
		return fmt.Errorf("failed to get recording path: %w", err)
	}

	// Delete from database
	query := `DELETE FROM gate_recordings WHERE id = $1`
	_, err = r.db.ExecContext(ctx, query, recordingID)
	if err != nil {
		return fmt.Errorf("failed to delete recording: %w", err)
	}

	// Delete physical file (ignore errors if file doesn't exist)
	_ = os.Remove(filePath)

	return nil
}

// ListRecordingsBySession returns all recordings for a session.
// Results are ordered by created_at DESC (most recent first).
func (r *RecordingService) ListRecordingsBySession(ctx context.Context, sessionID string) ([]*Recording, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, session_id, user_id, device_id, file_path, timing_path, byte_size, duration_seconds, started_at, completed_at, created_at
		FROM gate_recordings
		WHERE session_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list recordings for session: %w", err)
	}
	defer rows.Close()

	var recordings []*Recording
	for rows.Next() {
		rec := &Recording{}
		err := rows.Scan(
			&rec.ID, &rec.SessionID, &rec.UserID, &rec.DeviceID, &rec.FilePath,
			&rec.TimingPath, &rec.ByteSize, &rec.DurationSeconds, &rec.StartedAt,
			&rec.CompletedAt, &rec.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording row: %w", err)
		}
		recordings = append(recordings, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recordings: %w", err)
	}

	return recordings, nil
}

// ListRecordingsByDevice returns all recordings for a device.
// Results are ordered by created_at DESC (most recent first).
func (r *RecordingService) ListRecordingsByDevice(ctx context.Context, deviceID string) ([]*Recording, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT id, session_id, user_id, device_id, file_path, timing_path, byte_size, duration_seconds, started_at, completed_at, created_at
		FROM gate_recordings
		WHERE device_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list recordings for device: %w", err)
	}
	defer rows.Close()

	var recordings []*Recording
	for rows.Next() {
		rec := &Recording{}
		err := rows.Scan(
			&rec.ID, &rec.SessionID, &rec.UserID, &rec.DeviceID, &rec.FilePath,
			&rec.TimingPath, &rec.ByteSize, &rec.DurationSeconds, &rec.StartedAt,
			&rec.CompletedAt, &rec.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording row: %w", err)
		}
		recordings = append(recordings, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recordings: %w", err)
	}

	return recordings, nil
}
