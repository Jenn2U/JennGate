-- Add GUI session fields to gate_sessions
ALTER TABLE gate_sessions ADD COLUMN (
  enable_gui BOOLEAN DEFAULT FALSE,
  gui_protocol TEXT,
  x11_display_port INT,
  vnc_port INT,
  gui_session_started_at TIMESTAMP,
  gui_session_ended_at TIMESTAMP
);

-- Create index for GUI protocol queries
CREATE INDEX idx_sessions_gui_protocol ON gate_sessions(gui_protocol) WHERE gui_protocol IS NOT NULL;

-- Add new event types to audit log (for reference in code)
-- These are just documentation; actual event_type is a string column
COMMENT ON TABLE gate_audit_log IS 'Event types: device_approved, device_decommissioned, session_started, session_ended, gui_session_started, gui_session_ended, orphan_cleanup, policy_synced';
