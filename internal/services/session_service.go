package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionService manages remote access session lifecycle.
// It tracks session state transitions and metadata for SSH connections.
type SessionService struct {
	db *sql.DB
	mu sync.RWMutex
}

// Session represents a remote access session with state machine tracking.
type Session struct {
	ID               string
	UserID           string
	DeviceID         string
	State            string    // REQUESTED, AUTHORIZED, ACTIVE, DISCONNECTED
	CertSerial       string
	CertExpiresAt    time.Time
	StartedAt        time.Time
	ConnectedAt      *time.Time
	DisconnectedAt   *time.Time
	SSHPort          int
	RecordingID      *string
	DisconnectReason *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewSessionService creates a new SessionService with database connection.
func NewSessionService(db *sql.DB) *SessionService {
	return &SessionService{
		db: db,
	}
}

// CreateSession creates a new session in REQUESTED state.
// It assigns a UUID for the session ID and sets the StartedAt timestamp.
func (s *SessionService) CreateSession(
	ctx context.Context,
	userID string,
	deviceID string,
	certSerial string,
	certExpiresAt time.Time,
) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO gate_sessions (id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, ssh_port, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, created_at, updated_at
	`

	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		DeviceID:     deviceID,
		State:        "REQUESTED",
		CertSerial:   certSerial,
		CertExpiresAt: certExpiresAt,
		StartedAt:    now,
		SSHPort:      2222,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := s.db.QueryRowContext(
		ctx,
		query,
		sessionID, userID, deviceID, "REQUESTED", certSerial, certExpiresAt, now, 2222, now, now,
	).Scan(
		&session.ID, &session.UserID, &session.DeviceID, &session.State,
		&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
		&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
		&session.RecordingID, &session.DisconnectReason, &session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session from the database by ID.
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, created_at, updated_at
		FROM gate_sessions
		WHERE id = $1
	`

	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.UserID, &session.DeviceID, &session.State,
		&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
		&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
		&session.RecordingID, &session.DisconnectReason, &session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSessionState validates state transition and updates the state.
// Valid transitions: REQUESTED -> AUTHORIZED -> ACTIVE -> DISCONNECTED
func (s *SessionService) UpdateSessionState(ctx context.Context, sessionID string, newState string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Validate new state
	validStates := map[string]bool{
		"REQUESTED":    true,
		"AUTHORIZED":   true,
		"ACTIVE":       true,
		"DISCONNECTED": true,
	}

	if !validStates[newState] {
		return fmt.Errorf("invalid state: %s", newState)
	}

	// Get current state
	var currentState string
	err := s.db.QueryRowContext(ctx, "SELECT state FROM gate_sessions WHERE id = $1", sessionID).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Validate state transition
	validTransitions := map[string]map[string]bool{
		"REQUESTED":    {"AUTHORIZED": true, "DISCONNECTED": true},
		"AUTHORIZED":   {"ACTIVE": true, "DISCONNECTED": true},
		"ACTIVE":       {"DISCONNECTED": true},
		"DISCONNECTED": {},
	}

	transitions, ok := validTransitions[currentState]
	if !ok || !transitions[newState] {
		return fmt.Errorf("invalid state transition: %s -> %s", currentState, newState)
	}

	// Update state
	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET state = $1, updated_at = $2
		WHERE id = $3
	`

	_, err = s.db.ExecContext(ctx, query, newState, now, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	return nil
}

// MarkConnected sets ConnectedAt timestamp and transitions session to ACTIVE state.
// The session must be in AUTHORIZED state to transition to ACTIVE.
func (s *SessionService) MarkConnected(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get current state
	var currentState string
	err := s.db.QueryRowContext(ctx, "SELECT state FROM gate_sessions WHERE id = $1", sessionID).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get session state: %w", err)
	}

	// Only transition from AUTHORIZED to ACTIVE
	if currentState != "AUTHORIZED" {
		return fmt.Errorf("cannot mark connected: session must be in AUTHORIZED state, current state: %s", currentState)
	}

	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET connected_at = $1, state = $2, updated_at = $3
		WHERE id = $4
	`

	_, err = s.db.ExecContext(ctx, query, now, "ACTIVE", now, sessionID)
	if err != nil {
		return fmt.Errorf("failed to mark session connected: %w", err)
	}

	return nil
}

// DisconnectSession transitions session to DISCONNECTED state and sets disconnection metadata.
func (s *SessionService) DisconnectSession(ctx context.Context, sessionID string, reason string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get current state
	var currentState string
	err := s.db.QueryRowContext(ctx, "SELECT state FROM gate_sessions WHERE id = $1", sessionID).Scan(&currentState)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get session state: %w", err)
	}

	// Cannot transition from DISCONNECTED
	if currentState == "DISCONNECTED" {
		return fmt.Errorf("session already disconnected")
	}

	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET state = $1, disconnected_at = $2, disconnect_reason = $3, updated_at = $4
		WHERE id = $5
	`

	_, err = s.db.ExecContext(ctx, query, "DISCONNECTED", now, reason, now, sessionID)
	if err != nil {
		return fmt.Errorf("failed to disconnect session: %w", err)
	}

	return nil
}

// ListSessionsByDevice returns all sessions for a device (both active and recent).
func (s *SessionService) ListSessionsByDevice(ctx context.Context, deviceID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, created_at, updated_at
		FROM gate_sessions
		WHERE device_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions for device: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		err := rows.Scan(
			&session.ID, &session.UserID, &session.DeviceID, &session.State,
			&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
			&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
			&session.RecordingID, &session.DisconnectReason, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// ListActiveSessions returns only ACTIVE sessions for a device.
func (s *SessionService) ListActiveSessions(ctx context.Context, deviceID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, created_at, updated_at
		FROM gate_sessions
		WHERE device_id = $1 AND state = $2
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID, "ACTIVE")
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		err := rows.Scan(
			&session.ID, &session.UserID, &session.DeviceID, &session.State,
			&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
			&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
			&session.RecordingID, &session.DisconnectReason, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan active session row: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating active sessions: %w", err)
	}

	return sessions, nil
}

// CleanupExpiredSessions marks sessions as DISCONNECTED if their certificates have expired.
// This is intended to be run periodically (every 1 hour).
func (s *SessionService) CleanupExpiredSessions(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET state = $1, disconnected_at = $2, disconnect_reason = $3, updated_at = $4
		WHERE state != $5 AND cert_expires_at < $6
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		"DISCONNECTED",
		now,
		"cert_expired",
		now,
		"DISCONNECTED",
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return nil
}
