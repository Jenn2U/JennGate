# JennGate: Standalone Remote Access Service

JennGate is a standalone Go service that manages SSH certificate issuance, session lifecycle, and remote access for the Jenn ecosystem. It provides isolated hardware-based tamper-proofing, compiled daemons on edge devices, and secure remote access without exposing core infrastructure.

## Features

- **SSH Certificate Authority**: Ed25519 ephemeral certificate issuance with configurable TTL (default 1 hour)
- **Session Management**: Full state machine (REQUESTED → AUTHORIZED → ACTIVE → DISCONNECTED)
- **Session Recording**: Automatic capture of terminal sessions via `script(1)` wrapper
- **Device Registration**: Self-registration from daemons on first connect
- **Device Approval Flow**: Explicit approval + periodic orphan detection
- **REST API**: 11 endpoints for cert issuance, session management, device admin
- **WebSocket Terminal**: Interactive terminal access (Phase 3a: echo stub, Phase 3b: full SSH)
- **gRPC Daemon Interface**: Device communication (registration, policy sync, session reporting)
- **Audit Logging**: Comprehensive audit trail of all state changes

## GUI Access (VNC + X11)

Phase 3b extends JennGate with dual-protocol GUI access:

### SSH Terminal (Phase 3a)
- Text-based access via WebSocket terminal
- Recorded via `script(1)` typescript format

### VNC (Headless)
- Requires `enable_gui: true` in certificate request
- Accessed via SSH port forwarding
- Available on JennEdge and JennDock daemons
- No recording (real-time access only)

### X11 Forwarding (Workstations)
- Requires `enable_gui: true` and JennEdge daemon
- Accessed via SSH -X forwarding
- Optional Xvfb virtual display
- No recording (real-time access only)

### Feature Flag
- Set `enable_gui: true` in cert request
- Requires `gate.gui.access` permission
- Defaults to `false` (backward compatible)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       JennGate Service                           │
│                    (Go, port 8081/9090)                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ REST API │  │WebSocket │  │   gRPC   │  │    DB    │        │
│  │ (port    │  │ Terminal │  │  Daemon  │  │(Postgres)│        │
│  │ 8081)    │  │ (port    │  │ (port    │  │          │        │
│  │          │  │  8081)   │  │ 9090)    │  │          │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│      ▲              ▲              ▲              ▲              │
│      │              │              │              │              │
│      └──────────────┴──────────────┴──────────────┘              │
│                                                                   │
│  Services: CA, Session, Recording                                │
│  Auth: JWT (REST), mTLS (gRPC/Daemon)                            │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start (Docker)

### Prerequisites
- Docker & Docker Compose
- Port 8081 (HTTP API) and 5432 (PostgreSQL) available

### Local Development

```bash
# Clone the repository
git clone https://github.com/Jenn2U/JennGate.git
cd JennGate

# Start services
docker-compose up

# Verify health
curl http://localhost:8081/health

# Run tests
docker-compose exec jenngate go test ./internal/services -v
```

## Deployment

### Environment Variables

**Required:**
- `JENNGATE_DB_PASSWORD` - PostgreSQL password

**Optional (with defaults):**
- `JENNGATE_DB_HOST` - Default: `localhost`
- `JENNGATE_DB_PORT` - Default: `5432`
- `JENNGATE_DB_NAME` - Default: `jenngate`
- `JENNGATE_DB_USER` - Default: `jenngate`
- `JENNGATE_DB_SSLMODE` - Default: `require` (set to `disable` for local development)
- `JENNGATE_HTTP_PORT` - Default: `8081`
- `JENNGATE_LOG_LEVEL` - Default: `info` (options: debug, info, warn, error)
- `JENNGATE_RECORDING_DIR` - Default: `/var/lib/jenngate/recordings`

### Production Deployment

1. **Build Docker Image**
   ```bash
   docker build -t jenn2u/jenngate:v3.1.0 .
   docker push jenn2u/jenngate:v3.1.0
   ```

2. **Deploy to Production Host** (10.10.50.155)
   ```bash
   ssh root@10.10.50.155
   
   # Pull and run with Docker Compose
   docker pull jenn2u/jenngate:v3.1.0
   docker run -d \
     --name jenngate \
     --restart=always \
     -p 8081:8081 \
     -p 9090:9090 \
     -e JENNGATE_DB_PASSWORD=$DB_PASSWORD \
     -e JENNGATE_DB_HOST=postgres.local \
     -e JENNGATE_DB_SSLMODE=require \
     jenn2u/jenngate:v3.1.0
   ```

3. **Verify Deployment**
   ```bash
   curl https://jenn2u.ai/jenngate/health
   ```

## API Endpoints

### Health Check
- `GET /health` - Health status and database connectivity

### Certificate API
- `POST /api/v1/gate/cert/issue` - Issue SSH certificate
  ```json
  {
    "device_id": "device-uuid",
    "duration_minutes": 60
  }
  ```

### Session Management
- `GET /api/v1/gate/sessions` - List sessions (with filtering)
- `GET /api/v1/gate/sessions/:session_id` - Get session details

### Recordings
- `GET /api/v1/gate/recordings/:recording_id` - Download recording

### Device Admin
- `GET /admin/gate/pending-devices` - List pending devices
- `POST /admin/gate/devices/:device_id/approve` - Approve device
- `POST /admin/gate/devices/:device_id/decommission` - Decommission device

### WebSocket Terminal
- `WS /ws/gate/:session_id` - Interactive terminal access (Phase 3a: echo mode)

### gRPC Daemon Interface (port 9090)
- `RegisterDaemon` - Device daemon registration
- `ReportSessionStart` - Session start notification
- `ReportSessionEnd` - Session end notification
- `FetchPolicies` - Access policy synchronization

## Database Schema

### Tables
1. **devices** - Device registry with approval state
2. **gate_sessions** - Session state machine with lifecycle tracking
3. **gate_recordings** - Recording metadata and file paths
4. **gate_ca_keys** - SSH CA key storage (encrypted at rest)
5. **gate_audit_log** - Comprehensive audit trail

See `migrations/001_init_schema.up.sql` for full schema.

## Build from Source

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Make (optional)

### Build
```bash
# Development build
go build -o jenngate ./cmd/jenngate

# Production build (stripped)
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o jenngate ./cmd/jenngate

# Run
./jenngate
```

### Testing
```bash
# Unit tests (default database skip)
go test ./internal/services -v

# With test database
export JENNGATE_DB_HOST=localhost
export JENNGATE_DB_PASSWORD=jenngate
go test ./internal/services -v

# Integration tests
go test ./tests/integration -v
```

## Architecture Decisions

### Security
- **Ed25519 SSH Certificates**: Modern, resistant to quantum attacks
- **Ephemeral Certificates**: 1-hour TTL (configurable) limits exposure window
- **mTLS for Daemon Communication**: Mutual TLS prevents unauthorized device registration
- **Encrypted CA Keys**: Private key stored encrypted in PostgreSQL
- **Hardware Isolation**: Deployed on isolated network segment

### Scalability
- **Stateless Service**: All state in PostgreSQL, can run multiple instances
- **Connection Pooling**: MaxOpenConns=10, MaxIdleConns=5 for DB
- **Asynchronous Recording**: Non-blocking session capture via `script(1)`
- **Efficient Queries**: Indexed lookups by device/user/session

### Maintainability
- **Modular Services**: CA, Session, Recording, Discovery services are independent
- **Migrations**: golang-migrate with reversible migrations
- **Comprehensive Logging**: All operations logged to audit table
- **Clear Separation**: REST (HTTP), WebSocket, gRPC (daemon) endpoints separate

## Phase Roadmap

### Phase 3a: UI Migration & Core Infrastructure
- ✅ REST API (11 endpoints)
- ✅ WebSocket terminal (echo stub)
- ✅ gRPC daemon interface (stubs)
- ✅ Certificate issuance
- ✅ Session state machine
- ✅ Recording infrastructure

### Phase 3b (Current): Full SSH & Dual-Protocol GUI
- ✅ SSH daemon connection (full SSH, replaced echo mode)
- ✅ Policy sync CRDT
- ✅ Full gRPC implementation with protobuf
- ✅ JWT authentication & authorization
- ✅ VNC service for headless access (JennEdge/JennDock)
- ✅ X11 service for workstation access (JennEdge)
- ✅ GUI feature flag & permission (`gate.gui.access`)
- ✅ Database schema extended for GUI fields
- ✅ Orphan detection job
- ✅ Integration tests (14 pre-release validation checklist items)

## Contributing

1. Create a branch: `git checkout -b feature/my-feature`
2. Make changes with clear, focused commits
3. Write tests for new functionality
4. Run `go test ./...` and `go fmt ./...` before committing
5. Submit a pull request with clear description

## License

Proprietary - Jenn2U, Inc.

## Support

For issues, questions, or feature requests, contact the Jenn Infrastructure team.
