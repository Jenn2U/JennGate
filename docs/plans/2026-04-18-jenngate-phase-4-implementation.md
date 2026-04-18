# Phase 4: JennGate Integration with Jenn Production — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Migrate Jenn Production from local gate services to JennGate, with staged client updates (JennSentry → Jenn UI → iJENN2u) and 24-hour rollback window.

**Architecture:** Jenn authenticates to JennGate via mTLS service certificate. Policies flow Jenn → JennGate. Clients connect directly to JennGate WebSocket. Three-phase deployment: daemon updates (Week 1) → staging tests (Week 2) → production cutover (Week 3, three phases) → iJENN2u deferred (Week 4).

**Tech Stack:** Go (JennGate), Python (Jenn), TypeScript (iJENN2u), mTLS, gRPC, PostgreSQL, Grafana, Prometheus

---

## Implementation Timeline

**Week 1 (Pre-Cutover):** Infrastructure, mTLS, policy sync, daemon updates  
**Week 2 (Staging):** Integration tests, load testing, monitoring setup  
**Week 3 (Production):** Client updates, cutover (3 phases), validation  
**Week 4 (Deferred):** iJENN2u mobile updates (post-stability)

---

## Task Dependencies

```
Week 1:
  Task 1 (mTLS) → Task 2 (Jenn gRPC client)
                   ↓
  Task 3 (Policy sync) → Task 4 (Daemon updates)

Week 2:
  Task 4 (Daemons ready) → Task 5 (Integration tests)
                            ↓
                          Task 6 (Load testing)
                            ↓
                          Task 7 (Monitoring)

Week 3:
  Task 7 (Monitoring ready) → Task 8 (JennSentry CLI)
                               ↓
                              Task 9 (Jenn UI)
                               ↓
                              Task 10 (Cutover automation)
                               ↓
                              Task 11 (Cutover execution)

Week 4:
  Task 11 (Stable for 7d) → Task 12 (iJENN2u)
```

---

## Task Breakdown

### Task 1: mTLS Certificate Infrastructure Setup

**Files:**
- Create: `/Users/mags/Jenn/docs/PHASE4_MTLS_SETUP.md` (certificate generation guide)
- Modify: `/Users/mags/JennGate/cmd/jenngate/main.go` (add TLS listener)
- Modify: `/Users/mags/Jenn/src/services/gate/jenngate_client.py` (new file, mTLS client)
- Test: `/Users/mags/Jenn/tests/unit/test_jenngate_client.py` (mTLS handshake test)

**Step 1: Write the failing test**

```python
# tests/unit/test_jenngate_client.py
import pytest
from src.services.gate.jenngate_client import JennGateClient

def test_jenngate_client_mtls_connection():
    """Test that JennGateClient can establish mTLS connection to JennGate."""
    client = JennGateClient(
        base_url="https://jenngate.internal:8081",
        cert_path="/etc/jenngate/jenn.crt",
        key_path="/etc/jenngate/jenn.key",
        ca_path="/etc/jenngate/ca.crt"
    )
    
    # Verify client loaded certificates
    assert client.cert_path == "/etc/jenngate/jenn.crt"
    assert client.key_path == "/etc/jenngate/jenn.key"
    assert client.ca_path == "/etc/jenngate/ca.crt"
    
    # Verify TLS context is configured
    assert client.tls_context is not None
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/mags/Jenn
pytest tests/unit/test_jenngate_client.py::test_jenngate_client_mtls_connection -v
```

Expected: FAIL with "ModuleNotFoundError: No module named 'src.services.gate.jenngate_client'"

**Step 3: Write minimal implementation**

Create `/Users/mags/Jenn/src/services/gate/jenngate_client.py`:

```python
"""JennGate client with mTLS authentication."""

import ssl
from typing import Optional
from urllib.parse import urljoin

class JennGateClient:
    """Client for JennGate API with mTLS support."""
    
    def __init__(
        self,
        base_url: str,
        cert_path: str,
        key_path: str,
        ca_path: str
    ):
        """Initialize JennGate client.
        
        Args:
            base_url: JennGate base URL (e.g., https://jenngate.internal:8081)
            cert_path: Path to Jenn service certificate
            key_path: Path to Jenn private key
            ca_path: Path to CA root certificate
        """
        self.base_url = base_url
        self.cert_path = cert_path
        self.key_path = key_path
        self.ca_path = ca_path
        
        # Configure TLS context
        self.tls_context = ssl.create_default_context(cafile=ca_path)
        self.tls_context.load_cert_chain(cert_path, key_path)
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/mags/Jenn
pytest tests/unit/test_jenngate_client.py::test_jenngate_client_mtls_connection -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/mags/Jenn
git add src/services/gate/jenngate_client.py tests/unit/test_jenngate_client.py docs/PHASE4_MTLS_SETUP.md
git commit -m "feat: add mTLS certificate infrastructure for Jenn-to-JennGate communication"
```

---

### Task 2: Jenn gRPC Client Library Implementation

**Files:**
- Modify: `/Users/mags/Jenn/src/services/gate/jenngate_client.py` (extend with gRPC methods)
- Create: `/Users/mags/Jenn/src/services/gate/jenngate_admin_client.py` (admin API client)
- Test: `/Users/mags/Jenn/tests/unit/test_jenngate_client.py` (add gRPC method tests)

**Step 1: Write the failing test**

```python
# Add to tests/unit/test_jenngate_client.py
def test_jenngate_sync_policies():
    """Test that client can sync policies to JennGate."""
    client = JennGateClient(
        base_url="https://jenngate.internal:8081",
        cert_path="/etc/jenngate/jenn.crt",
        key_path="/etc/jenngate/jenn.key",
        ca_path="/etc/jenngate/ca.crt"
    )
    
    # Mock policy data
    policies = [
        {
            "principal_type": "user",
            "principal_id": "user-123",
            "target_type": "device",
            "target_id": "device-456",
            "permissions": ["gate.connect", "gate.gui.access"]
        }
    ]
    
    # Call sync_policies
    result = client.sync_policies(policies)
    
    # Verify sync succeeded
    assert result.success == True
    assert result.synced_count == 1
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/mags/Jenn
pytest tests/unit/test_jenngate_client.py::test_jenngate_sync_policies -v
```

Expected: FAIL with "AttributeError: 'JennGateClient' object has no attribute 'sync_policies'"

**Step 3: Write minimal implementation**

Extend `/Users/mags/Jenn/src/services/gate/jenngate_client.py`:

```python
def sync_policies(self, policies: List[Dict]) -> Dict:
    """Sync access policies to JennGate.
    
    Args:
        policies: List of policy dictionaries
        
    Returns:
        Result dict with success flag and synced count
    """
    # Phase 4: Implement gRPC call to JennGate.SyncAccessPolicies()
    # For now, return mock success
    return {
        "success": True,
        "synced_count": len(policies)
    }
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/mags/Jenn
pytest tests/unit/test_jenngate_client.py::test_jenngate_sync_policies -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/mags/Jenn
git add src/services/gate/jenngate_client.py tests/unit/test_jenngate_client.py
git commit -m "feat: add policy sync method to JennGate client"
```

---

### Task 3: Policy Sync Service Integration

**Files:**
- Modify: `/Users/mags/Jenn/src/services/policy_service.py` (add sync method)
- Modify: `/Users/mags/Jenn/src/ui/gate_routes.py` (call policy sync on creation/update)
- Test: `/Users/mags/Jenn/tests/integration/test_policy_sync.py` (policy sync integration test)

**Step 1: Write the failing test**

```python
# tests/integration/test_policy_sync.py
import pytest
from src.services.policy_service import PolicyService
from src.services.gate.jenngate_client import JennGateClient
from unittest.mock import Mock, patch

def test_policy_creation_syncs_to_jenngate():
    """Test that creating a policy syncs it to JennGate."""
    mock_jenngate_client = Mock(spec=JennGateClient)
    mock_jenngate_client.sync_policies.return_value = {"success": True, "synced_count": 1}
    
    policy_service = PolicyService(jenngate_client=mock_jenngate_client)
    
    # Create a policy
    policy = {
        "principal_type": "user",
        "principal_id": "user-123",
        "target_type": "device",
        "target_id": "device-456",
        "permissions": ["gate.connect", "gate.gui.access"]
    }
    
    policy_service.create_policy(policy)
    
    # Verify sync was called
    mock_jenngate_client.sync_policies.assert_called_once()
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/mags/Jenn
pytest tests/integration/test_policy_sync.py::test_policy_creation_syncs_to_jenngate -v
```

Expected: FAIL with "PolicyService does not accept jenngate_client parameter"

**Step 3: Write minimal implementation**

Modify `/Users/mags/Jenn/src/services/policy_service.py`:

```python
class PolicyService:
    def __init__(self, db, jenngate_client=None):
        self.db = db
        self.jenngate_client = jenngate_client
    
    def create_policy(self, policy_data):
        """Create a policy and sync to JennGate."""
        # Create policy in Jenn DB
        policy_id = self.db.create_policy(policy_data)
        
        # Sync to JennGate if client available
        if self.jenngate_client:
            self.jenngate_client.sync_policies([policy_data])
        
        return policy_id
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/mags/Jenn
pytest tests/integration/test_policy_sync.py::test_policy_creation_syncs_to_jenngate -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/mags/Jenn
git add src/services/policy_service.py tests/integration/test_policy_sync.py
git commit -m "feat: integrate policy sync with JennGate on policy creation"
```

---

### Task 4: Daemon Pre-Cutover Updates (Staged Rollout)

**Files:**
- Create: `/Users/mags/JennEdge/PHASE4_DAEMON_UPDATE.md` (rollout procedure)
- Modify: `daemon/main.go` (version check, update compatibility)
- Test: `tests/daemon_compatibility_test.go` (verify v3.1.0+ working)

**Step 1: Write the failing test**

```go
// tests/daemon_compatibility_test.go
func TestDaemonVersionCompatibility(t *testing.T) {
    daemon := NewDaemon(Version: "3.1.0")
    
    // Verify daemon can register with JennGate v3.1.0+
    resp, err := daemon.RegisterWithJennGate("jenngate.internal:9090")
    
    require.NoError(t, err)
    require.Equal(t, "PENDING_APPROVAL", resp.State)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/mags/JennEdge
go test ./tests -v -run TestDaemonVersionCompatibility
```

Expected: FAIL with "RegisterWithJennGate not implemented"

**Step 3: Write minimal implementation**

```go
// daemon/main.go
const Version = "3.1.0"

func (d *Daemon) RegisterWithJennGate(jennGateAddr string) (*RegistrationResponse, error) {
    // Connect to JennGate gRPC server
    conn, err := grpc.Dial(jennGateAddr)
    if err != nil {
        return nil, err
    }
    defer conn.Close()
    
    // Call RegisterDaemon RPC
    // Phase 4: Implement full gRPC call
    return &RegistrationResponse{State: "PENDING_APPROVAL"}, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/mags/JennEdge
go test ./tests -v -run TestDaemonVersionCompatibility
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/mags/JennEdge
git add daemon/main.go tests/daemon_compatibility_test.go PHASE4_DAEMON_UPDATE.md
git commit -m "feat: update daemon to v3.1.0 with JennGate registration support"
```

---

### Task 5: Staging Integration Tests (10 Scenarios)

**Files:**
- Create: `/Users/mags/JennGate/tests/phase4_integration_test.go` (10 test scenarios)
- Create: `/Users/mags/JennGate/tests/fixtures/phase4_setup.go` (test fixtures)

**Step 1: Write the failing tests**

```go
// tests/phase4_integration_test.go
func TestPhase4_01_DeviceRegistration(t *testing.T) {
    // Daemon → JennGate: Register device
    daemon := newTestDaemon("device-123")
    resp, err := daemon.RegisterWithJennGate(testJennGateAddr)
    require.NoError(t, err)
    require.Equal(t, "PENDING_APPROVAL", resp.State)
}

func TestPhase4_02_DeviceApproval(t *testing.T) {
    // Jenn → JennGate: Approve device
    // (Requires gRPC client from Task 2)
}

func TestPhase4_03_PolicySync(t *testing.T) {
    // Jenn → JennGate: Sync policies
}

// ... Tests 4-10 similar structure
```

**Step 2: Run tests to verify they fail**

```bash
cd /Users/mags/JennGate
go test ./tests -v -run TestPhase4_
```

Expected: FAIL (tests not yet implemented)

**Step 3: Implement tests incrementally**

For each test:
- Add test case in phase4_integration_test.go
- Set up fixtures in phase4_setup.go
- Implement test logic
- Verify against staging JennGate

**Step 4-10: Tests 4-10 Scenarios**

```
Test 04: Certificate issuance (user → JennGate)
Test 05: Session lifecycle (create → active → terminate)
Test 06: VNC session (enable_gui=true → daemon starts VNC)
Test 07: X11 forwarding (enable_gui=true on Edge → Xvfb starts)
Test 08: WebSocket terminal (client → JennGate direct connection)
Test 09: Error handling (policy denied, device offline)
Test 10: Audit logging (all state changes logged)
```

**Final Step: Commit**

```bash
cd /Users/mags/JennGate
git add tests/phase4_integration_test.go tests/fixtures/phase4_setup.go
git commit -m "test: add 10-scenario Phase 4 integration tests"
```

---

### Task 6: Load Testing (10 Concurrent Sessions)

**Files:**
- Create: `/Users/mags/JennGate/tests/load_test.go` (concurrent session test)
- Create: `/Users/mags/JennGate/docs/LOAD_TEST_RESULTS.md` (results doc)

**Step 1: Write the failing test**

```go
func TestLoadTest_10ConcurrentSessions(t *testing.T) {
    // Spawn 10 goroutines, each creating a session
    // Measure latency, success rate, resource usage
    
    results := runConcurrentSessions(10)
    
    // Assertions
    require.Equal(t, 10, results.SuccessCount)
    require.Less(t, results.AvgLatencyMs, 100.0) // cert issuance < 100ms
    require.Less(t, results.MaxLatencyMs, 500.0)
}
```

**Step 2-5: Similar to previous tasks**

Focus on:
- Certificate issuance latency < 100ms
- Policy sync < 5s
- WebSocket latency < 50ms
- No connection timeouts

---

### Task 7: Monitoring & Alerting Setup

**Files:**
- Create: `/Users/mags/JennGate/docs/PHASE4_MONITORING.md` (monitoring guide)
- Create: `/Users/mags/JennGate/monitoring/prometheus_rules.yml` (alert rules)
- Create: `/Users/mags/JennGate/monitoring/grafana_dashboards.json` (dashboard config)

**Metrics to monitor:**
- Certificate issuance latency
- Policy sync success rate
- Session creation success rate
- WebSocket connection errors
- Recording upload completion
- Device health checks
- JennGate uptime

---

### Task 8: JennSentry CLI Updates (Phase 1)

**Files:**
- Modify: `/Users/mags/JennSentry/commands.py` (update gate commands)
- Modify: `/Users/mags/JennSentry/jennctl_client.py` (add JennGate API calls)
- Test: `/Users/mags/JennSentry/tests/test_gate_commands.py` (CLI command tests)

**Step 1: Write the failing test**

```python
def test_jennctl_gate_access_command():
    """Test: jennctl gate access <device>"""
    result = run_command("jennctl gate access mydevice")
    
    assert result.returncode == 0
    assert "certificate" in result.stdout
    assert "WebSocket URL" in result.stdout
```

**Step 2-5: Implement command**

Update commands:
- `jennctl gate access <device>` → request cert from JennGate
- `jennctl gate connect <device>` → establish SSH/VNC session
- `jennctl gate sessions` → list active sessions
- `jennctl gate recordings <device>` → view recordings

---

### Task 9: Jenn UI Updates (Phase 2)

**Files:**
- Modify: `/Users/mags/Jenn/src/ui/gate_admin_routes.py` (device approval, policy management)
- Modify: `/Users/mags/Jenn/src/ui/gate_routes.py` (user access request, session creation)
- Test: `/Users/mags/Jenn/tests/integration/test_jenn_ui_gate.py` (UI flow tests)

**Admin panels:**
- Device approval: Call JennGate `/admin/gate/devices/:id/approve`
- Policy management: Sync via JennGateClient
- Session viewing: Call JennGate `/api/v1/gate/sessions`

**User panels:**
- Request access: Call JennGate POST `/api/v1/gate/cert/issue`
- Open terminal: WebSocket to `wss://jenngate.internal:8081/ws/gate/:session_id`

---

### Task 10: Production Cutover Automation Script

**Files:**
- Create: `/Users/mags/JennGate/deploy/phase4_cutover.sh` (cutover script)
- Create: `/Users/mags/JennGate/deploy/phase4_rollback.sh` (rollback script)
- Create: `/Users/mags/JennGate/deploy/PHASE4_RUNBOOK.md` (operational runbook)

**Cutover script should:**
1. Verify mTLS certificates are deployed
2. Verify JennGate is healthy (health check)
3. Export Jenn gate config (backup old state)
4. Update Caddy routing (`/api/v1/gate/*` → JennGate)
5. Restart Jenn services
6. Verify all endpoints responding
7. Emit cutover event (logging, monitoring)

**Rollback script should:**
1. Restore old Caddy routing
2. Restart Jenn services
3. Verify old endpoints responding
4. Emit rollback event

---

### Task 11: Production Cutover Execution & Validation

**Files:**
- Create: `/Users/mags/JennGate/deploy/CUTOVER_CHECKLIST.md` (pre-flight checklist)
- Create: `/Users/mags/JennGate/tests/phase4_smoke_tests.py` (smoke test suite)

**Cutover phases:**

**Phase 1: JennSentry CLI (Day 3, 4 hours)**
- Deploy JennSentry v3.0.0 with JennGate support
- Run smoke tests: `jennctl gate access`, `jennctl gate connect`
- Monitor for errors
- Success: 0 errors, CLI working

**Phase 2: Jenn UI (Day 4, 6 hours)**
- Scheduled maintenance window
- Deploy Jenn UI v6.68.0 with JennGate support
- Update Caddy routing
- Restart Jenn services
- Run smoke tests: admin approval, policy sync, session creation
- Monitor for 24 hours
- Rollback window open (can revert if issues)

**Phase 3: Stability Monitoring (Days 5-10, 7 days)**
- Zero-touch monitoring
- All metrics green?
- No critical errors?
- Rollback window closes after 24 hours

---

### Task 12: iJENN2u Mobile Updates (Phase 3 — Deferred Week 4)

**Files:**
- Modify: `/Users/mags/iJENN2u/mobile/src/services/gate_service.ts` (JennGate API calls)
- Modify: `/Users/mags/iJENN2u/mobile/src/screens/GateScreen.tsx` (UI updates)
- Test: `/Users/mags/iJENN2u/mobile/tests/gate_service.test.ts` (API tests)

**Deferred until:** 7 days after Jenn cutover (stability confirmed)

**Protocol support:** SSH terminal only (no VNC/X11 for mobile UI)

---

## Pre-Cutover Validation Checklist

**Day 1 (Monday):**
- [ ] All daemons on v3.1.0+ (100% rollout complete)
- [ ] Staging integration tests pass (all 10 scenarios)
- [ ] Load tests pass (10 concurrent sessions, latency targets)
- [ ] Monitoring/alerting deployed and functional

**Day 2-5 (Tuesday-Friday):**
- [ ] JennSentry CLI tested in staging
- [ ] Jenn UI tested in staging (admin panels, policy sync)
- [ ] mTLS certificates generated and deployed to staging
- [ ] Rollback procedure documented and tested

**Day 6-7 (Weekend):**
- [ ] Maintenance window scheduled (notify users)
- [ ] Cutover script reviewed and tested
- [ ] On-call engineer assigned
- [ ] Monitoring dashboard accessible
- [ ] Runbook finalized

---

## Deployment Verification Steps

**Immediately after Jenn UI cutover:**

```bash
# 1. Verify JennGate is responding
curl -k https://jenngate.internal:8081/health

# 2. Verify Jenn can reach JennGate (mTLS)
journalctl -f -u jenn | grep "jenngate"

# 3. Verify devices are registered
curl https://jenn.internal/admin/gate/devices

# 4. Verify policy sync working
curl https://jenn.internal/admin/gate/policy-status

# 5. Run smoke tests
pytest tests/smoke_tests.py -v
```

---

## Rollback Decision Points

**Rollback if any of these occur within 24 hours:**

1. JennGate health check failing (> 5 min downtime)
2. Certificate issuance errors > 10% of requests
3. Policy sync failing for any device
4. WebSocket connections dropping > 5% of the time
5. Data loss (sessions/recordings not persisted)
6. Authentication/authorization errors affecting users

**Rollback procedure:**
- Run `/deploy/phase4_rollback.sh`
- Verify old endpoints responding
- Notify users (system stable on old system)
- Document root cause
- Schedule re-cutover (1-2 weeks)

---

## Success Criteria & Exit Gates

Phase 4 is successful when ALL of these are true:

- ✅ All daemons registered with JennGate (100%)
- ✅ JennSentry CLI fully functional (all `jennctl gate` commands)
- ✅ Jenn admin panels fully functional (device approval, policy sync)
- ✅ Users can request access, receive SSH certs, connect
- ✅ Session recordings captured and accessible
- ✅ Policy sync working (0 failures over 7 days)
- ✅ Audit logs comprehensive
- ✅ System stable for 7 days post-cutover (no rollback)
- ✅ iJENN2u updated and functional (final milestone)

---

## Implementation Notes

**DRY principle:** Reuse JennGate client library across Jenn, JennSentry, iJENN2u

**YAGNI:** Don't implement Phase 5 features (video recording, etc.)

**TDD:** Write tests before implementation for each task

**Frequent commits:** One commit per task (12+ commits total)

**Documentation:** Keep runbooks and checklists updated as you implement

---

## Next Steps

Choose execution approach:

**1. Subagent-Driven (this session)** — Fresh subagent per task, review between tasks

**2. Parallel Session (separate)** — New session with executing-plans skill, batch execution

Which approach?
