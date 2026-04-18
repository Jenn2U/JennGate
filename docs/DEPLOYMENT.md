# JennGate Deployment Guide

This document covers deployment of JennGate to production and development environments.

## Environment Variables

### Core Configuration

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

### GUI Services (Phase 3b)

**VNC Configuration:**
- `DAEMON_ENABLE_VNC` - Enable VNC service (default: true)
- `DAEMON_VNC_PORT` - VNC port (default: 5900)

**X11 Configuration:**
- `DAEMON_ENABLE_X11` - Enable X11 service (default: true on JennEdge, false on JennDock)
- `DAEMON_X11_RESOLUTION` - Xvfb resolution (default: 1280x720)
  - Common values: 1280x720, 1920x1080, 2560x1440

## Deployment Procedures

### Local Development

1. **Start PostgreSQL and JennGate**
   ```bash
   docker-compose up
   ```

2. **Verify Health**
   ```bash
   curl http://localhost:8081/health
   ```

### Production Deployment (10.10.50.155)

1. **Build Docker Image**
   ```bash
   docker build -t jenn2u/jenngate:v3.1.0 .
   docker push jenn2u/jenngate:v3.1.0
   ```

2. **SSH to Production Host**
   ```bash
   ssh root@10.10.50.155
   ```

3. **Pull Latest Image**
   ```bash
   docker pull jenn2u/jenngate:v3.1.0
   ```

4. **Run Container with GUI Services**
   ```bash
   docker run -d \
     --name jenngate \
     --restart=always \
     -p 8081:8081 \
     -p 9090:9090 \
     -p 5900:5900 \
     -e JENNGATE_DB_PASSWORD=$DB_PASSWORD \
     -e JENNGATE_DB_HOST=postgres.local \
     -e JENNGATE_DB_SSLMODE=require \
     -e DAEMON_ENABLE_VNC=true \
     -e DAEMON_ENABLE_X11=true \
     -e DAEMON_X11_RESOLUTION=1920x1080 \
     jenn2u/jenngate:v3.1.0
   ```

5. **Verify Deployment**
   ```bash
   curl https://jenn2u.ai/jenngate/health
   ```

### Environment-Specific Configuration

**JennEdge Deployment:**
```bash
docker run -d \
  --name jenngate-edge \
  --restart=always \
  -p 8081:8081 \
  -p 9090:9090 \
  -p 5900:5900 \
  -e DAEMON_ENABLE_VNC=true \
  -e DAEMON_ENABLE_X11=true \
  -e DAEMON_X11_RESOLUTION=1920x1080 \
  jenn2u/jenngate:v3.1.0
```

**JennDock Deployment:**
```bash
docker run -d \
  --name jenngate-dock \
  --restart=always \
  -p 8081:8081 \
  -p 9090:9090 \
  -p 5900:5900 \
  -e DAEMON_ENABLE_VNC=true \
  -e DAEMON_ENABLE_X11=false \
  jenn2u/jenngate:v3.1.0
```

## Health Checks

### HTTP Health Endpoint
```bash
curl -s http://localhost:8081/health | jq .
```

### Database Connectivity
Health endpoint includes PostgreSQL connectivity status.

### Service Status
```bash
# Check running container
docker ps | grep jenngate

# View logs
docker logs jenngate

# Check resource usage
docker stats jenngate
```

## Troubleshooting

### Database Connection Errors
- Verify `JENNGATE_DB_PASSWORD` is correct
- Check PostgreSQL is accessible at `JENNGATE_DB_HOST:JENNGATE_DB_PORT`
- Confirm `JENNGATE_DB_SSLMODE` matches PostgreSQL configuration

### VNC Access Issues
- Verify `DAEMON_ENABLE_VNC=true`
- Check port 5900 is accessible
- Ensure certificate request includes `enable_gui: true`
- Verify user has `gate.gui.access` permission

### X11 Forwarding Issues
- Only available on JennEdge (set `DAEMON_ENABLE_X11=true`)
- Requires `enable_gui: true` in certificate request
- SSH must be invoked with `-X` flag
- Xvfb may need additional display server configuration

## Monitoring & Logging

### Log Levels
Set `JENNGATE_LOG_LEVEL` for verbosity:
- `debug` - Detailed operational logs
- `info` - Standard operational information
- `warn` - Warning messages only
- `error` - Error messages only

### Audit Trail
All state changes are logged to `gate_audit_log` table in PostgreSQL. Query examples:
```sql
-- Recent device approvals
SELECT * FROM gate_audit_log WHERE event_type = 'DEVICE_APPROVED' ORDER BY created_at DESC LIMIT 10;

-- Session lifecycle events
SELECT * FROM gate_audit_log WHERE event_type LIKE 'SESSION_%' ORDER BY created_at DESC LIMIT 20;

-- GUI access attempts
SELECT * FROM gate_audit_log WHERE event_type LIKE 'GUI_%' ORDER BY created_at DESC LIMIT 10;
```

## Backward Compatibility

JennGate v3.1.0 is fully backward compatible with v3.0.0:
- `enable_gui` defaults to `false`
- Existing SSH workflows unchanged
- Phase 3a features fully functional without GUI configuration
- Old clients work without modification
