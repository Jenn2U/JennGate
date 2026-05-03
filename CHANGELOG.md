# Changelog

All notable changes to JennGate are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.1.1] - 2026-05-03

### Added
- **Restored CI workflows** (`.github/workflows/`) that were lost when main was force-pushed during repo creation on 2026-04-17:
  - `check-tests.yml` — Go tests against PostgreSQL 15 service container with 90% coverage gate (mirrors JennAuth pattern)
  - `build-jenngate.yml` — multi-arch Docker build, push to `ghcr.io/jenn2u/jenngate` on main + tag events (PR builds verify Dockerfile only)
- Coverage attribution uses `-coverpkg=./internal/...,./cmd/...` so centralized tests in `tests/unit/` and `tests/integration/` credit coverage to the packages they exercise.

### Notes
- Coverage gate will initially fail (Phase 4 work has 12 test files but no prior coverage threshold). Closing the gap is tracked separately.
- Phase 4 design doc references CI/CD-deployed GitHub Org Secrets — restoring the workflows unblocks that work stream.

## [3.1.0] - 2026-04-18

### Added
- Dual-protocol GUI access (VNC + X11)
- New `enable_gui` parameter in certificate request
- New `gate.gui.access` RBAC permission
- VNC service for headless access (JennEdge/JennDock)
- X11 service for workstation access (JennEdge)
- Session status endpoint: GET /api/v1/gate/sessions/:id/status
- 14-item pre-release validation checklist
- Environment variables for GUI service configuration
  - `DAEMON_ENABLE_VNC`
  - `DAEMON_ENABLE_X11`
  - `DAEMON_X11_RESOLUTION`
  - `DAEMON_VNC_PORT`

### Changed
- Extended `gate_sessions` table with GUI fields
- SessionService now tracks GUI protocol and port
- PolicyService evaluates `gate.gui.access` permission
- Updated database schema in migrations/001_init_schema.up.sql

### Backward Compatibility
- `enable_gui` defaults to `false`
- Old clients unaffected
- Existing SSH workflows unchanged
- Phase 3a features fully preserved

## [3.0.0] - 2026-04-17

### Added
- SSH Certificate Authority with Ed25519 ephemeral certificates
- Session Management with full state machine (REQUESTED → AUTHORIZED → ACTIVE → DISCONNECTED)
- Session Recording via `script(1)` wrapper
- Device Registration and Approval Flow
- REST API with 11 endpoints for cert issuance and session management
- WebSocket Terminal access (Phase 3a: echo stub)
- gRPC Daemon Interface for device communication
- Comprehensive Audit Logging
- Docker Compose deployment configuration
- PostgreSQL schema with 5 core tables
- golang-migrate database migrations

### Architecture
- Stateless service design with PostgreSQL backend
- Ed25519 SSH certificates with 1-hour TTL (configurable)
- mTLS for daemon communication
- Encrypted CA keys at rest
- Hardware isolation on dedicated network segment
