package models

// IssueCertRequest is the request to issue a new SSH certificate.
type IssueCertRequest struct {
	DeviceID        string `json:"device_id" binding:"required"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	EnableGUI       bool   `json:"enable_gui"`
}

// ListSessionsRequest is the request to list sessions (query parameters).
type ListSessionsRequest struct {
	UserID   string `form:"user_id"`
	DeviceID string `form:"device_id"`
	State    string `form:"state"`
	Limit    int    `form:"limit,default=100"`
	Offset   int    `form:"offset,default=0"`
}

// ApprovDeviceRequest is the request to approve a pending device.
type ApproveDeviceRequest struct {
	ApproverID string `json:"approver_id" binding:"required"`
}

// DecommissionDeviceRequest is the request to decommission a device.
type DecommissionDeviceRequest struct {
	DecommissionerID string `json:"decommissioner_id" binding:"required"`
	Reason           string `json:"reason"`
}

// ReportSessionStartRequest is reported by daemon when session starts.
type ReportSessionStartRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	SSHPort   int    `json:"ssh_port"`
}

// ReportSessionEndRequest is reported by daemon when session ends.
type ReportSessionEndRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Reason    string `json:"reason"`
}
