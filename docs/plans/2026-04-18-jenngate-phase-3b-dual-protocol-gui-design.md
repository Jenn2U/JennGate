# JennGate Phase 3b: Dual-Protocol GUI Access (SSH Terminal + VNC + X11)

**Date:** 2026-04-18  
**Status:** Design Approved  
**Decision:** Unified Session Model (Single Certificate, Protocol Flexibility)

---

## Context

JennGate Phase 3a (UI Migration) is complete with 11 REST endpoints, WebSocket terminal stub, gRPC daemon interface, and core services (CA, Session, Recording). Phase 3b extends this with **full remote access capability**: SSH terminal, VNC (headless), and X11 (workstations) in a single unified session model.

---

## Architecture: Unified Session Model

**Single SSH Certificate, Multiple Protocol Access:**

```
User Request (Jenn/JennSentry/iJENN2u)
    ↓
JennGate: Issue Ephemeral Ed25519 Certificate (1 hour TTL)
    ↓
Session Created (REQUESTED → AUTHORIZED → ACTIVE)
    ↓
    ├─ Protocol: SSH Terminal
    │  └─ WebSocket Bridge: Client ↔ SSH/2222
    │
    ├─ Protocol: VNC (headless daemons)
    │  └─ VNC Server: 5900 (local only, port forwarded via SSH ProxyJump)
    │  └─ Daemon: JennEdge/JennDock listening on 5900
    │
    └─ Protocol: X11 (workstation daemons)
       └─ X11 Server: localhost:10 (local only, forwarded via SSH -X)
       └─ Daemon: Xvfb/Wayland on JennEdge (optional)
```

**Key Design Principles:**

1. **Single Certificate for All Protocols** — One Ed25519 cert validates user for all access types
2. **Port Forwarding, Not Direct Access** — VNC/X11 accessed via SSH ProxyJump/X11 forwarding, not exposed to network
3. **Text Recording Only (SSH)** — Terminal sessions recorded via script(1) typescript; VNC/X11 sessions not recorded (no storage overhead)
4. **Protocol Opt-In** — `enable_gui` boolean flag in session creation; defaults to `false` for backward compatibility
5. **Daemon-Side Implementation** — VNC/X11 server logic lives on JennEdge/JennDock daemons, not JennGate

---

## Components: Extended Session & Daemon Services

### 1. Session Service (Extended)

**New Fields in `gate_sessions` Table:**
```sql
ALTER TABLE gate_sessions ADD COLUMN (
  enable_gui BOOLEAN DEFAULT FALSE,        -- User wants VNC/X11 access
  gui_protocol TEXT,                       -- 'vnc', 'x11', or NULL
  x11_display_port INT,                    -- e.g., 6010 for localhost:10
  vnc_port INT,                            -- e.g., 5900
  gui_session_started_at TIMESTAMP,        -- When VNC/X11 was actually opened
  gui_session_ended_at TIMESTAMP           -- When VNC/X11 was closed
);
```

**New Methods in SessionService:**
- `UpdateSessionGUIStatus(ctx, sessionID, protocol, displayPort, vncPort)` — Called by daemon after VNC/X11 server starts
- `EndGUISession(ctx, sessionID)` — Called when user disconnects VNC/X11

**State Machine Extended:**
```
REQUESTED ─────→ AUTHORIZED ─────→ ACTIVE
                                    ├─ SSH Terminal: ACTIVE
                                    ├─ GUI (if enable_gui=true):
                                    │  ├─ WAITING_FOR_GUI (daemon spinning up VNC/X11)
                                    │  └─ GUI_ACTIVE (both SSH + GUI available)
                                    └─ DISCONNECTED (any protocol closes)
```

### 2. New VNC Service on Daemons (JennEdge/JennDock)

**Daemon-Side Component: `daemon/vnc_service.go`**

- Start VNC server on port 5900 when user connects with `enable_gui=true`
- VNC validates incoming connections against JennGate-issued certificate
- Supports TightVNC/RealVNC protocols
- Listens on localhost only (accessed via SSH port forwarding)
- Stops VNC server when session ends

**RPC to JennGate:**
```proto
rpc NotifyGUISessionStart(GUISessionStartRequest) returns (Empty);
message GUISessionStartRequest {
  string session_id = 1;
  string protocol = 2;  // "vnc" or "x11"
  int32 port = 3;
}
```

### 3. New X11 Service on Daemons (JennEdge)

**Daemon-Side Component: `daemon/x11_service.go`** (for workstation daemons only)

- Start Xvfb (X Virtual Framebuffer) on `:10` (port 6010)
- X11 forwarding validated via SSH certificate
- Supports X11 protocol tunneling through SSH `-X` forwarding
- Optional: GNOME/KDE window managers for full desktop
- Stops Xvfb when session ends

**Not activated on JennDock (headless variant)** — VNC only.

### 4. Policy Service (Extended)

**New RBAC Permission:**
```
gate.gui.access — User can request GUI (VNC/X11) access
```

**Policy Evaluation Updated:**
```python
def can_access_gui(user_id, device_id, policy_cache):
    """Check if user has gate.gui.access permission for device."""
    return policy_cache.evaluate(
        principal=(user_id, 'user'),
        resource=(device_id, 'device'),
        permission='gate.gui.access'
    )
```

**CRDT Policy Fields:**
```
permissions: ['gate.connect', 'gate.record.view', 'gate.gui.access']
max_gui_concurrent_sessions: 1  # Prevent concurrent GUI on same device
```

---

## REST API Changes (Phase 3b)

**Modified POST `/api/v1/gate/cert/issue`**

```json
Request:
{
  "device_id": "device-uuid",
  "duration_minutes": 60,
  "enable_gui": false  // NEW: Optional, defaults to false
}

Response:
{
  "cert_pem": "-----BEGIN CERTIFICATE-----...",
  "key_pem": "-----BEGIN PRIVATE KEY-----...",
  "expires_at": "2026-04-18T19:00:00Z",
  "session_id": "session-uuid",
  "ssh_port": 2222,
  "gui_available": false,  // NEW: true if enable_gui + user has permission
  "vnc_port": null,        // NEW: 5900 if VNC configured
  "x11_display": null      // NEW: "localhost:10" if X11 configured
}
```

**New Status Endpoint: GET `/api/v1/gate/sessions/:session_id/status`**

```json
Response:
{
  "session_id": "uuid",
  "state": "ACTIVE",
  "ssh_active": true,
  "gui_active": false,
  "gui_protocol": null,
  "gui_port": null,
  "ssh_port": 2222,
  "started_at": "...",
  "updated_at": "..."
}
```

**Backward Compatibility:**
- Old clients omit `enable_gui` → defaults to `false` → works as before
- New clients can request `enable_gui: true` → approved/denied by policy
- Old response parsing: `gui_available` and GUI port fields gracefully ignored

---

## Testing: Pre-Release Validation (14 Items)

**Unit Tests (Services):**
1. ✅ SessionService.UpdateSessionGUIStatus() updates DB correctly
2. ✅ VNCService.StartVNCServer() binds to port 5900
3. ✅ X11Service.StartXvfb() binds to :10 (6010)
4. ✅ PolicyService.can_access_gui() evaluates permissions

**Integration Tests (Daemon ↔ JennGate):**
5. ✅ Daemon registers with JennGate
6. ✅ Session created with enable_gui=true
7. ✅ Daemon receives NotifyGUISessionStart RPC
8. ✅ VNC server starts on daemon, port confirmed
9. ✅ Session status endpoint returns gui_active=true
10. ✅ User can SSH into 2222, port forwarding to 5900 works

**E2E Tests (User Workflows):**
11. ✅ SSH Terminal: User connects, runs commands, disconnects (Phase 3a flow unchanged)
12. ✅ VNC Access: User requests enable_gui=true, connects to VNC via SSH tunnel
13. ✅ Policy Enforcement: User without gate.gui.access gets enable_gui=false in response
14. ✅ Clean Shutdown: Session ends → VNC server stops, no orphaned processes

---

## Recording Strategy (No Change from Phase 3a)

**Why Text SSH Only:**
- VNC/X11 recording requires video codec (H.264/VP8) → 50-100 MB/month per session overhead
- Text transcripts sufficient for audit (commands, output, user actions)
- Video compression/playback adds JennGate complexity (transcoding, browser support)
- Compliance: Most audit requirements satisfied by text (what command? who ran it? result?)

**Implementation:**
- SSH terminal: `script(1)` wrapper captures typescript (unchanged)
- VNC/X11: No recording (users see GUI in real-time, no archive)
- Rationale: Supports remote work (GUI for interactive tasks) without storage burden

---

## Daemon Changes: Extended `gate-daemon` Binary

**New Go Packages in Daemon:**
```
daemon/vnc_service.go      — Start/stop VNC server on 5900
daemon/x11_service.go      — Start/stop Xvfb on :10 (JennEdge only)
daemon/gui_handler.go      — Handle NotifyGUISessionStart RPC
```

**New Environment Variables (Optional):**
```
DAEMON_ENABLE_VNC=true          # Default: true (JennDock + JennEdge)
DAEMON_ENABLE_X11=true          # Default: true (JennEdge), false (JennDock)
DAEMON_X11_RESOLUTION=1920x1080 # Default: 1280x720
DAEMON_VNC_PORT=5900            # Default: 5900
```

**Startup Sequence:**
1. Daemon registers with JennGate (unchanged)
2. Wait for policies (unchanged)
3. If DAEMON_ENABLE_VNC=true: Start VNC service (listening, not accepting yet)
4. If DAEMON_ENABLE_X11=true: Start Xvfb service (listening, not accepting yet)
5. On NotifyGUISessionStart RPC: Enable VNC/X11 for that session only

---

## Deployment & Rollout

**Phase 3b Release (Pre-Release/Staging Only):**
- Deploy JennGate with extended session schema + API changes
- Deploy daemon binaries with VNC/X11 services
- Internal testing: SSH + VNC + X11 workflows
- No Phase 4 integration yet (Jenn still calls old stub APIs)
- 14-item validation checklist must pass before Phase 4 Jenn integration

**Backward Compatibility:**
- Existing SSH-only workflows unaffected (enable_gui defaults to false)
- Old clients work without changes
- New clients can opt-in to GUI access with explicit enable_gui=true

---

## Success Criteria

✅ Users can request `enable_gui=true` in certificate request  
✅ Daemon starts VNC server when requested  
✅ VNC accessible via SSH port forwarding (localhost:5900)  
✅ X11 accessible via SSH -X forwarding (JennEdge only)  
✅ Session state machine tracks GUI protocol  
✅ PolicyService enforces gate.gui.access permission  
✅ SSH terminal workflow unchanged (backward compatible)  
✅ No GUI recording (text SSH only, no storage overhead)  
✅ 14-item pre-release validation checklist passes  
✅ Zero security vulnerabilities (certificate validation for VNC/X11)  

---

## Next Steps

**Phase 3b Implementation Plan:**
- Invoke writing-plans skill to create detailed implementation plan
- ~10 tasks: DB schema migration, SessionService extension, VNCService, X11Service, PolicyService update, API changes, daemon integration, testing, deployment validation

**Phase 4: Jenn Production Integration**
- Migrate `gate_routes.py` from Jenn to JennGate APIs
- Update JennSentry CLI (`gate` commands)
- Update iJENN2u mobile client
- Full end-to-end integration testing
- Production deployment to 10.10.50.155
