-- Rollback GUI session fields
DROP INDEX IF EXISTS idx_sessions_gui_protocol;

-- PostgreSQL requires individual ALTER TABLE statements for each column
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS enable_gui;
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS gui_protocol;
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS x11_display_port;
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS vnc_port;
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS gui_session_started_at;
ALTER TABLE gate_sessions DROP COLUMN IF EXISTS gui_session_ended_at;
