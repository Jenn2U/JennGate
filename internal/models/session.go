package models

import "time"

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
	TerminatedAt        *time.Time
	SSHPort             int
	ProxyChain          string
	RecordingID         *string
	DisconnectReason    *string
	// GUI session fields
	EnableGUI           bool       // NEW
	GUIProtocol         *string    // NEW: "vnc", "x11", or nil
	X11DisplayPort      *int       // NEW
	VNCPort             *int       // NEW
	GUISessionStartedAt *time.Time // NEW
	GUISessionEndedAt   *time.Time // NEW
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
