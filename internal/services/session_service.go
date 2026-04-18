package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionState represents the state of a session in the lifecycle
type SessionState string

const (
	StateRequested   SessionState = "REQUESTED"
	StateAuthorized  SessionState = "AUTHORIZED"
	StateActive      SessionState = "ACTIVE"
	StateDisconnected SessionState = "DISCONNECTED"
)

// SessionService manages remote access session lifecycle.
// It tracks session state transitions and metadata for SSH connections.
type SessionService struct {
	db *sql.DB
	mu sync.RWMutex
}

// Session represents a remote access session with state machine tracking.
type Session struct {
	ID                  string
	UserID              string
	DeviceID            string
	State               string    // REQUESTED, AUTHORIZED, ACTIVE, DISCONNECTED
	CertSerial          string
	CertExpiresAt       time.Time
	StartedAt           time.Time
	ConnectedAt         *time.Time
	DisconnectedAt      *time.Time
	SSHPort             int
	RecordingID         *string
	DisconnectReason    *string
	GUIProtocol         *string    // "vnc", "x11", or nil
	X11DisplayPort      *int
	VNCPort             *int
	GUISessionStartedAt *time.Time
	GUISessionEndedAt   *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
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
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO gate_sessions (id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, ssh_port, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, gui_protocol, x11_display_port, vnc_port, gui_session_started_at, gui_session_ended_at, created_at, updated_at
	`

	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		DeviceID:     deviceID,
		State:        string(StateRequested),
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
		sessionID, userID, deviceID, StateRequested, certSerial, certExpiresAt, now, 2222, now, now,
	).Scan(
		&session.ID, &session.UserID, &session.DeviceID, &session.State,
		&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
		&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
		&session.RecordingID, &session.DisconnectReason,
		&session.GUIProtocol, &session.X11DisplayPort, &session.VNCPort,
		&session.GUISessionStartedAt, &session.GUISessionEndedAt,
		&session.CreatedAt, &session.UpdatedAt,
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
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, gui_protocol, x11_display_port, vnc_port, gui_session_started_at, gui_session_ended_at, created_at, updated_at
		FROM gate_sessions
		WHERE id = $1
	`

	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.UserID, &session.DeviceID, &session.State,
		&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
		&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
		&session.RecordingID, &session.DisconnectReason,
		&session.GUIProtocol, &session.X11DisplayPort, &session.VNCPort,
		&session.GUISessionStartedAt, &session.GUISessionEndedAt,
		&session.CreatedAt, &session.UpdatedAt,
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
		string(StateRequested):   true,
		string(StateAuthorized):  true,
		string(StateActive):      true,
		string(StateDisconnected): true,
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
		string(StateRequested):   {string(StateAuthorized): true, string(StateDisconnected): true},
		string(StateAuthorized):  {string(StateActive): true, string(StateDisconnected): true},
		string(StateActive):      {string(StateDisconnected): true},
		string(StateDisconnected): {},
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
	if currentState != string(StateAuthorized) {
		return fmt.Errorf("cannot mark connected: session must be in AUTHORIZED state, current state: %s", currentState)
	}

	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET connected_at = $1, state = $2, updated_at = $3
		WHERE id = $4
	`

	_, err = s.db.ExecContext(ctx, query, now, StateActive, now, sessionID)
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
	if currentState == string(StateDisconnected) {
		return fmt.Errorf("session already disconnected")
	}

	now := time.Now()
	query := `
		UPDATE gate_sessions
		SET state = $1, disconnected_at = $2, disconnect_reason = $3, updated_at = $4
		WHERE id = $5
	`

	_, err = s.db.ExecContext(ctx, query, StateDisconnected, now, reason, now, sessionID)
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
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, gui_protocol, x11_display_port, vnc_port, gui_session_started_at, gui_session_ended_at, created_at, updated_at
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
			&session.RecordingID, &session.DisconnectReason,
			&session.GUIProtocol, &session.X11DisplayPort, &session.VNCPort,
			&session.GUISessionStartedAt, &session.GUISessionEndedAt,
			&session.CreatedAt, &session.UpdatedAt,
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
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, gui_protocol, x11_display_port, vnc_port, gui_session_started_at, gui_session_ended_at, created_at, updated_at
		FROM gate_sessions
		WHERE device_id = $1 AND state = $2
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID, StateActive)
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
			&session.RecordingID, &session.DisconnectReason,
			&session.GUIProtocol, &session.X11DisplayPort, &session.VNCPort,
			&session.GUISessionStartedAt, &session.GUISessionEndedAt,
			&session.CreatedAt, &session.UpdatedAt,
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

// ListSessionsByUser returns all sessions for a specific user
func (s *SessionService) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, user_id, device_id, state, cert_serial, cert_expires_at, started_at, connected_at, disconnected_at, ssh_port, recording_id, disconnect_reason, gui_protocol, x11_display_port, vnc_port, gui_session_started_at, gui_session_ended_at, created_at, updated_at
		FROM gate_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions for user: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		err := rows.Scan(
			&session.ID, &session.UserID, &session.DeviceID, &session.State,
			&session.CertSerial, &session.CertExpiresAt, &session.StartedAt,
			&session.ConnectedAt, &session.DisconnectedAt, &session.SSHPort,
			&session.RecordingID, &session.DisconnectReason,
			&session.GUIProtocol, &session.X11DisplayPort, &session.VNCPort,
			&session.GUISessionStartedAt, &session.GUISessionEndedAt,
			&session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
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
		StateDisconnected, // $1
		now,               // $2
		"cert_expired",    // $3
		now,               // $4
		StateDisconnected, // $5
		now,               // $6
	)

	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return nil
}

// UpdateSessionGUIStatus updates the session with GUI protocol and port information.
// Called by daemon when VNC/X11 server starts.
func (s *SessionService) UpdateSessionGUIStatus(ctx context.Context,
	sessionID, protocol string, vncPort, x11DisplayPort int) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE gate_sessions
		SET gui_protocol = $1,
		    vnc_port = $2,
		    x11_display_port = $3,
		    gui_session_started_at = NOW()
		WHERE id = $4
	`

	_, err := s.db.ExecContext(ctx, query, protocol, vncPort, x11DisplayPort, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update GUI status: %w", err)
	}

	return nil
}

// EndGUISession clears GUI session data and records end time.
// Called when user disconnects VNC/X11.
func (s *SessionService) EndGUISession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE gate_sessions
		SET gui_protocol = NULL,
		    vnc_port = NULL,
		    x11_display_port = NULL,
		    gui_session_ended_at = NOW()
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end GUI session: %w", err)
	}

	return nil
}
