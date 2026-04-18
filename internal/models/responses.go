package models

import "time"

// IssueCertResponse returns the issued SSH certificate.
type IssueCertResponse struct {
	CertPEM   string    `json:"cert_pem"`
	KeyPEM    string    `json:"key_pem"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionInfo represents a single session for API responses.
type SessionInfo struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	DeviceID       string     `json:"device_id"`
	State          string     `json:"state"`
	CertSerial     *string    `json:"cert_serial"`
	CertExpiresAt  *time.Time `json:"cert_expires_at"`
	StartedAt      *time.Time `json:"started_at"`
	ConnectedAt    *time.Time `json:"connected_at"`
	DisconnectedAt *time.Time `json:"disconnected_at"`
	DisconnectReason *string  `json:"disconnect_reason"`
	RecordingID    *string    `json:"recording_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ListSessionsResponse returns a paginated list of sessions.
type ListSessionsResponse struct {
	Sessions []*SessionInfo `json:"sessions"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
}

// RecordingInfo represents recording metadata.
type RecordingInfo struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"session_id"`
	UserID          string     `json:"user_id"`
	DeviceID        string     `json:"device_id"`
	FilePath        string     `json:"file_path"`
	ByteSize        *int64     `json:"byte_size"`
	DurationSeconds *int       `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// DeviceInfo represents a device for API responses.
type DeviceInfo struct {
	ID              string     `json:"id"`
	DeviceName      string     `json:"device_name"`
	DeviceType      *string    `json:"device_type"`
	State           string     `json:"state"`
	DaemonVersion   *string    `json:"daemon_version"`
	ApprovedAt      *time.Time `json:"approved_at"`
	DecommissionedAt *time.Time `json:"decommissioned_at"`
	IsOrphaned      bool       `json:"is_orphaned"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ListPendingDevicesResponse returns pending devices awaiting approval.
type ListPendingDevicesResponse struct {
	Devices []*DeviceInfo `json:"devices"`
	Total   int           `json:"total"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// HealthResponse is the health check response.
type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

// ApproveDeviceResponse is returned when a device is approved.
type ApproveDeviceResponse struct {
	DeviceID       string `json:"device_id"`
	State          string `json:"state"`
	PoliciesSynced bool   `json:"policies_synced"`
}

// DecommissionDeviceResponse is returned when a device is decommissioned.
type DecommissionDeviceResponse struct {
	DeviceID           string `json:"device_id"`
	SessionsTerminated int    `json:"sessions_terminated"`
	RecordingsDeleted  int    `json:"recordings_deleted"`
	AuditLogged        bool   `json:"audit_logged"`
}
