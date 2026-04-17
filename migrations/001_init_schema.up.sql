-- Create devices table
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_name TEXT NOT NULL,
    device_type TEXT NOT NULL,
    state TEXT NOT NULL,
    approved_at TIMESTAMP,
    decommissioned_at TIMESTAMP,
    decommissioned_by TEXT,
    daemon_version TEXT,
    public_key_pem TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_devices_state ON devices(state);

-- Create gate_sessions table
CREATE TABLE gate_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- user_id references Jenn Production users table (external, not local FK)
    user_id UUID NOT NULL,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    cert_serial TEXT,
    cert_expires_at TIMESTAMP,
    started_at TIMESTAMP NOT NULL,
    connected_at TIMESTAMP,
    disconnected_at TIMESTAMP,
    ssh_port INTEGER DEFAULT 2222,
    recording_id UUID,
    disconnect_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gate_sessions_user_device ON gate_sessions(user_id, device_id);
CREATE INDEX idx_gate_sessions_state ON gate_sessions(state);

-- Create gate_recordings table
CREATE TABLE gate_recordings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES gate_sessions(id) ON DELETE CASCADE,
    -- user_id references Jenn Production users table (external, not local FK)
    user_id UUID NOT NULL,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    timing_path TEXT,
    byte_size INTEGER,
    duration_seconds INTEGER,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gate_recordings_user_device ON gate_recordings(user_id, device_id);

-- Create gate_ca_keys table
CREATE TABLE gate_ca_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_type TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    private_key_pem_encrypted TEXT NOT NULL,
    key_serial TEXT NOT NULL,
    rotated_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create gate_audit_log table
CREATE TABLE gate_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    actor_id TEXT,
    actor_type TEXT,
    resource_type TEXT,
    resource_id TEXT,
    details JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_gate_audit_log_resource ON gate_audit_log(resource_type, resource_id);
CREATE INDEX idx_gate_audit_log_event_type ON gate_audit_log(event_type);
