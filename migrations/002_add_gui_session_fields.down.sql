-- Rollback GUI session fields
DROP INDEX IF EXISTS idx_sessions_gui_protocol;

ALTER TABLE gate_sessions DROP COLUMN IF EXISTS (
  enable_gui,
  gui_protocol,
  x11_display_port,
  vnc_port,
  gui_session_started_at,
  gui_session_ended_at
);
