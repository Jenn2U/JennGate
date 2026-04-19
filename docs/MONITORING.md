# JennGate Production Monitoring & Alerting

**Version:** 1.0  
**Last Updated:** 2026-04-19  
**Status:** Phase 4 (Production Ready)

This document describes the observability infrastructure for JennGate in production: Prometheus metrics, alert rules, Grafana dashboards, and operational runbooks.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prometheus Metrics](#prometheus-metrics)
3. [Alert Rules & Thresholds](#alert-rules--thresholds)
4. [Grafana Dashboard](#grafana-dashboard)
5. [Alert Runbooks](#alert-runbooks)
6. [Troubleshooting Guide](#troubleshooting-guide)
7. [Scaling Recommendations](#scaling-recommendations)

---

## Architecture Overview

JennGate monitoring consists of three layers:

```
┌─────────────────────────────────────────────────────┐
│  JennGate Service (Prometheus-instrumented)         │
│  - gRPC server with gRPC metrics                     │
│  - Custom business metrics (cert, policy, session)   │
│  - Daemon heartbeat tracking                         │
└────────────────┬────────────────────────────────────┘
                 │ :9090/metrics (scrape endpoint)
                 ▼
┌─────────────────────────────────────────────────────┐
│  Prometheus Server (10.10.50.155:9090)              │
│  - Scrapes JennGate metrics every 30s               │
│  - Evaluates alert rules every 30s                  │
│  - Stores metrics in time-series database           │
│  - Config: prometheus.yml (scrape + rules)          │
└────────────────┬────────────────────────────────────┘
                 │ Query API
                 ▼
┌─────────────────────────────────────────────────────┐
│  Grafana (10.10.50.155:3000)                        │
│  - Dashboards (Dashboards/JennGate Phase 4)         │
│  - Alert notifications (PagerDuty, Slack, email)    │
└─────────────────────────────────────────────────────┘
```

### Key Files

- **Prometheus Rules:** `/Users/mags/JennGate/monitoring/prometheus_rules.yaml`
- **Grafana Dashboard:** `/Users/mags/JennGate/monitoring/grafana_dashboard.json`
- **Prometheus Config:** `/etc/prometheus/prometheus.yml` (on production host)

---

## Prometheus Metrics

### Baseline Performance (from Phase 4 Load Test)

All metrics collected during 10 concurrent user load test on 2026-04-19:

| Metric | Min | Avg | P99 | Max | Target |
|--------|-----|-----|-----|-----|--------|
| **cert_issuance_duration_seconds** | 8ms | 42ms | 79ms | 90ms | avg < 100ms, p99 < 150ms |
| **policy_sync_duration_seconds** | 1ms | 9ms | 45ms | 92ms | avg < 5s |
| **policy_sync_errors_total** | 0% | 0% | — | 0% | < 1% failure rate |
| **websocket_session_duration_seconds** | 0.1ms | 3.5ms | 15ms | 22ms | avg < 50ms, p99 < 100ms |
| **websocket_setup_time** | 146ms | 287ms | — | 512ms | avg < 1s |

### Certificate Issuance Metrics

```
Metric: jenngate_cert_issuance_duration_seconds
Type: Histogram (buckets: 0.01, 0.05, 0.1, 0.25, 0.5, 1.0)
Labels: None
Description: Time (seconds) to issue SSH certificate
Baseline: avg 42ms, p99 78ms
Alert: High if p99 > 250ms for 5min
Alert: Warn if avg > 150ms for 10min
```

**How to query:**
```promql
# Current request rate (certificates per second)
rate(jenngate_cert_issuance_duration_seconds_count[5m])

# P50, P99 latency
histogram_quantile(0.50, rate(jenngate_cert_issuance_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(jenngate_cert_issuance_duration_seconds_bucket[5m]))

# Average latency
rate(jenngate_cert_issuance_duration_seconds_sum[5m]) / rate(jenngate_cert_issuance_duration_seconds_count[5m])
```

### Policy Sync Metrics

```
Metric: jenngate_policy_sync_duration_seconds
Type: Histogram (buckets: 0.001, 0.01, 0.1, 1.0, 10.0)
Labels: None
Description: Time (seconds) to sync access policy from Jenn to JennGate
Baseline: avg 9ms, p99 45ms
Alert: High if p99 > 10s for 5min
Alert: Warn if error_rate > 1% for 5min
```

```
Metric: jenngate_policy_sync_errors_total
Type: Counter
Labels: error_type (policy_not_found, db_write_error, validation_error)
Description: Total policy sync failures
Alert: High if rate(error) > 0.01 (1%) for 5min
```

### WebSocket Terminal Metrics

```
Metric: jenngate_websocket_session_duration_seconds
Type: Histogram (buckets: 0.001, 0.01, 0.1, 0.5, 1.0)
Labels: None
Description: Time (seconds) from SSH command sent to response received
Baseline: avg 3.5ms, p99 15ms
Alert: Warn if p99 > 250ms for 5min
```

```
Metric: jenngate_websocket_connections_active
Type: Gauge
Labels: None
Description: Currently open WebSocket connections to terminals
Alert: Warn if > 1000 for 5min (resource exhaustion risk)
```

### Session Lifecycle Metrics

```
Metric: jenngate_session_state_transitions_total
Type: Counter
Labels: source (REQUESTED, AUTHORIZED, ACTIVE, TERMINATED, FAILED), target (next_state)
Description: Count of state transitions (e.g., REQUESTED → FAILED = session creation failure)
Alert: High if failures > 5 in 5min window
```

```
Metric: jenngate_session_duration_seconds
Type: Histogram
Labels: None
Description: Time (seconds) from session creation to termination
Alert: Warn if p95 > 86400 (24 hours, may indicate stuck sessions)
```

### Device Management Metrics

```
Metric: jenngate_devices_registered_total
Type: Counter
Labels: None
Description: Total devices registered with JennGate
Trend metric (always increasing); useful for tracking onboarding rate
```

```
Metric: jenngate_devices_pending_approval
Type: Gauge
Labels: None
Description: Devices currently in PENDING_APPROVAL state
Alert: Warn if > 10 for 1 hour (approval backlog)
```

```
Metric: jenngate_devices_approved_total
Type: Counter
Labels: None
Description: Total devices approved by administrators
```

### Recording & Storage Metrics

```
Metric: jenngate_recording_upload_duration_seconds
Type: Histogram
Labels: destination (nfs, sftp)
Description: Time (seconds) to upload session recording to NAS
Alert: Warn if p95 > 300s (5 minutes, NAS latency issue)
```

```
Metric: jenngate_recording_size_bytes
Type: Histogram
Labels: None
Description: File size (bytes) of session recordings
Trend metric; useful for capacity planning
```

### Storage Metrics

```
Metric: jenngate_storage_used_gb
Type: Gauge
Labels: mount_point
Description: Current NAS storage usage (GB)
Baseline: Varies per environment (tracks mount_point)
Units: Gigabytes (GB)
Alert: Warn if (used / total) > 80% for 5min
Alert: Critical if (used / total) > 95% for 1min
Dashboard Panel: Panel 6 (NAS Storage Utilization)
```

```
Metric: jenngate_storage_total_gb
Type: Gauge
Labels: mount_point
Description: Total NAS storage capacity (GB)
Baseline: Configured per deployment (e.g., 1000 GB, 5000 GB)
Units: Gigabytes (GB)
Used in ratio: (storage_used_gb / storage_total_gb) * 100 = %
```

### Daemon Connectivity Metrics

```
Metric: jenngate_daemon_connections_total
Type: Counter
Labels: daemon_id, status (connected, registered, failed)
Description: Total daemon registration attempts
```

```
Metric: jenngate_daemon_last_heartbeat_seconds_ago
Type: Gauge
Labels: daemon_id
Description: Time (seconds) since last heartbeat from daemon
Alert: Critical if > 300 (5 min) → daemon offline
Alert: Warn if > 120 (2 min) → daemon slow/unreachable
```

### Standard gRPC Metrics (Prometheus client library)

```
Metric: grpc_server_handled_total
Type: Counter
Labels: grpc_service, grpc_method, grpc_code (OK, UNKNOWN, INVALID_ARGUMENT, etc.)
Description: Total gRPC requests handled (per RPC method, per status code)
Alert: Error rate > 1% for 5min
```

```
Metric: grpc_server_handling_seconds
Type: Histogram (buckets: 0.001 to 10)
Labels: grpc_service, grpc_method
Description: gRPC request latency (seconds)
Alert: P99 > 1s for 5min
```

---

## Alert Rules & Thresholds

All alert rules are defined in `prometheus_rules.yaml`. Alert definitions use [Prometheus alert syntax](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/).

### Critical Alerts (page on-call immediately)

1. **JennGateCertIssuanceLatencyHigh** (P99 > 250ms for 5min)
   - Indicates CA service degradation
   - Check: CPU, database connection pool, OpenSSL performance
   - Estimated impact: Users unable to get SSH certificates

2. **JennGatePolicySyncErrorRateHigh** (error rate > 1% for 5min)
   - Policy delivery from Jenn to JennGate failing
   - Check: Jenn connectivity, mTLS certs, policy validation
   - Estimated impact: New policies not reaching JennGate

3. **JennGateSessionCreationFailures** (5+ failures in 5min)
   - Certificate issuance or authorization failures
   - Check: Certificate service, policy database, device approval
   - Estimated impact: Users cannot create new sessions

4. **JennGateDaemonOffline** (no heartbeat > 5min)
   - Daemon unreachable or crashed
   - Check: Daemon process status, network connectivity
   - Estimated impact: Device unable to accept connections

5. **JennGateStorageQuotaCritical** (> 95% for 1min)
   - NAS almost full, recordings may fail
   - Check: Disk usage, archival process, emergency cleanup
   - Estimated impact: Recording uploads will fail

6. **JennGateHealthCheckFailing** (service down for 1min)
   - JennGate service not responding
   - Check: Process status, port availability, logs
   - Estimated impact: All gate functionality offline

7. **JennGategRPCErrorRateHigh** (error rate > 1% for 5min)
   - RPC service errors accumulating
   - Check: Application logs, database, policy validation
   - Estimated impact: Random request failures

### Warning Alerts (notify ops, Slack channel)

1. **JennGateCertIssuanceLatencyWarn** (avg > 150ms for 10min)
2. **JennGatePolicySyncLatencyHigh** (p99 > 10s for 5min)
3. **JennGateWebSocketLatencyHigh** (p99 > 250ms for 5min)
4. **JennGateWebSocketConnectionCountHigh** (> 1000 for 5min)
5. **JennGateDaemonHeartbeatWarn** (> 2min since last heartbeat)
6. **JennGateRecordingUploadSlow** (p95 > 5min)
7. **JennGateStorageQuotaHigh** (> 80% for 5min)
8. **JennGateDevicesPendingApprovalStuck** (> 10 devices for 1 hour)
9. **JennGategRPCLatencyHigh** (p99 > 1s for 5min)
10. **JennGateRestartFrequent** (> 30min intervals)

---

## Grafana Dashboard

The dashboard `/Users/mags/JennGate/monitoring/grafana_dashboard.json` contains 8 panels:

### Panel 1: Certificate Issuance Latency (top-left)
- **Type:** Time series line graph
- **Metrics:** P50, P99, Average, Max, Min
- **Thresholds:** Green < 100ms, Yellow > 150ms, Red > 250ms
- **Baseline:** Avg 42ms, P99 78ms
- **Interpretation:**
  - Normal: P99 < 150ms
  - Warning: P99 100-250ms (approaching alert threshold)
  - Critical: P99 > 250ms (alert firing)

### Panel 2: Policy Sync Performance (top-right)
- **Type:** Time series line graph (dual axis)
- **Metrics Left:** Latency (avg milliseconds)
- **Metrics Right:** Error rate (%)
- **Thresholds:** Latency Green < 5000ms, Yellow > 5000ms, Red > 10000ms
- **Baseline:** Avg 9ms, Error rate 0%
- **Interpretation:**
  - Normal: Latency < 5s, Error rate < 0.1%
  - Warning: Latency > 5s OR error rate increasing
  - Critical: Error rate > 1%

### Panel 3: Active WebSocket Sessions (bottom-left)
- **Type:** Pie chart (current value gauge)
- **Metric:** `jenngate_websocket_connections_active`
- **Threshold:** Green < 500, Yellow 500-1000, Red > 1000
- **Interpretation:** Resource usage indicator; watch for sudden spikes

### Panel 4: Active Sessions Over Time (middle)
- **Type:** Time series area graph
- **Metric:** Session duration over time
- **Interpretation:** Trend of session activity; look for anomalies

### Panel 5: Daemon Health Table (center)
- **Type:** Table (instant values)
- **Columns:** Daemon ID, Seconds Since Last Heartbeat
- **Color scheme:** Green < 120s, Yellow 120-300s, Red > 300s
- **Interpretation:** 
  - Red rows = offline daemons (alert likely firing)
  - Yellow rows = sluggish daemons (warning firing)
  - Green rows = healthy

### Panel 6: NAS Storage Utilization (bottom-left)
- **Type:** Time series line graph
- **Metric:** Percentage used
- **Thresholds:** Green < 50%, Yellow 50-80%, Red > 80%
- **Interpretation:**
  - Green: Plenty of capacity
  - Yellow: Monitor closely, plan expansion
  - Red: Action required (alert firing)

### Panel 7: gRPC Error Rate (bottom-right)
- **Type:** Time series line graph
- **Metric:** Percentage errors
- **Threshold:** Green < 0.5%, Yellow 0.5-1%, Red > 1%
- **Interpretation:**
  - Baseline: < 0.1% in steady state
  - Yellow: Sporadic failures
  - Red: Systemic issue (alert firing)

### Panel 8: Session Lifecycle (bottom, full-width)
- **Type:** Time series bar graph (stacked)
- **Metrics:** Created (green), Terminated (blue), Failed (red)
- **Interpretation:**
  - Created ≈ Terminated (healthy churn)
  - Red bars = failures accumulating (alert likely firing)
  - Look for correlation with latency spikes

### How to Import Dashboard

In Grafana:
1. Click **Dashboards** → **New** → **Import**
2. Paste the JSON from `grafana_dashboard.json`
3. Select **Prometheus** as data source
4. Click **Import**

---

## Alert Runbooks

### Certificate Issuance Latency High

**Alert:** `JennGateCertIssuanceLatencyHigh` (P99 > 250ms)  
**Severity:** Critical  
**Estimated Impact:** Users unable to request SSH certificates; gate functionality degraded

**Diagnosis Steps:**

1. **Check CA service health:**
   ```bash
   # SSH to JennGate host
   ssh -i /path/to/key ops@jenngate.internal
   
   # Check JennGate logs for certificate generation errors
   journalctl -u jenngate -n 50 --no-pager | grep -i cert
   
   # Check CPU and disk I/O
   top -b -n 1 | head -20
   iostat -x 1 5
   ```

2. **Check database connection pool:**
   ```bash
   # SSH to JennGate host
   ssh -i /path/to/key ops@jenngate.internal
   
   # Check PostgreSQL connections
   psql -h localhost -U jenngate_app -d jenngate -c "SELECT count(*) FROM pg_stat_activity WHERE datname='jenngate';"
   
   # Look for locks
   psql -h localhost -U jenngate_app -d jenngate -c "SELECT * FROM pg_locks WHERE NOT granted;"
   ```

3. **Check Prometheus metrics:**
   - Navigate to Grafana → JennGate Phase 4 dashboard
   - Look at "Certificate Issuance Latency" panel
   - Check if P99 is trending up or spiking

4. **Check OpenSSL performance (if CA is local):**
   ```bash
   # Time a single certificate generation
   time openssl x509 -req -in test.csr -CA /etc/jenngate/ca.crt -CAkey /etc/jenngate/ca.key -out test.crt
   ```

**Resolution Steps:**

1. **If CPU high:**
   - Reduce load (pause non-critical tasks)
   - Scale JennGate horizontally (add replica)
   - Check for runaway process: `ps aux | grep jenngate`

2. **If database locks:**
   - Check for long-running transactions: `SELECT * FROM pg_stat_activity WHERE state='active';`
   - Kill blocking transaction if safe: `SELECT pg_terminate_backend(pid);`
   - Check indexes on certificate_requests table

3. **If disk I/O high:**
   - Check disk space: `df -h /`
   - Check I/O wait: `iostat -x` (look for high %iowait)
   - If full, move old logs/archives offline

4. **If issue persists:**
   - Restart JennGate service:
     ```bash
     sudo systemctl restart jenngate
     ```
   - Monitor P99 latency for next 10 minutes
   - If still high, escalate to on-call engineer

**Success Criteria:** P99 latency < 250ms sustained for 10 minutes

---

### Policy Sync Errors High

**Alert:** `JennGatePolicySyncErrorRateHigh` (error rate > 1%)  
**Severity:** Critical  
**Estimated Impact:** New policies not reaching JennGate; users lose access unexpectedly

**Diagnosis Steps:**

1. **Check Jenn connectivity:**
   ```bash
   # From JennGate host
   curl -k https://jenn.internal:8000/health
   
   # Check mTLS certificate validity
   openssl x509 -in /etc/jenngate/jenn-client.crt -noout -dates
   ```

2. **Check policy sync logs:**
   ```bash
   # View last 100 policy sync errors
   journalctl -u jenngate -n 100 --no-pager | grep -i "policy.*error\|sync.*fail"
   ```

3. **Check error types in Prometheus:**
   ```promql
   # View error rate by type
   rate(jenngate_policy_sync_errors_total[5m]) by (error_type)
   ```

4. **Verify mTLS certificates:**
   ```bash
   # Check certificate expiration
   openssl x509 -in /etc/jenngate/jenn-client.crt -noout -dates
   openssl x509 -in /etc/jenngate/jenn-client.key -noout -dates
   
   # Test mTLS handshake
   openssl s_client -cert /etc/jenngate/jenn-client.crt -key /etc/jenngate/jenn-client.key -CAfile /etc/jenngate/ca.crt -connect jenn.internal:8081
   ```

**Resolution Steps:**

1. **If certificate expired:**
   - Regenerate certificates: See `docs/PHASE4_MTLS_SETUP.md`
   - Restart JennGate: `sudo systemctl restart jenngate`
   - Verify policy sync error rate drops

2. **If Jenn unreachable:**
   - Check Jenn status: `curl -k https://jenn.internal:8000/health`
   - Check network connectivity: `ping jenn.internal`
   - Check firewall: `sudo ufw status` (allow 8081)
   - Restart Jenn if needed: Contact Jenn on-call

3. **If policy validation errors:**
   - Check Prometheus for `error_type="validation_error"`
   - Review policy format: Should have principal_type, target_type, permissions
   - Check Jenn logs for validation details

4. **If database write errors:**
   - Check PostgreSQL status: `pg_isready -h localhost -p 5432`
   - Check disk space: `df -h /`
   - Check table locks: `SELECT * FROM pg_locks WHERE NOT granted;`

**Success Criteria:** Error rate < 0.1% sustained for 15 minutes

---

### Daemon Offline

**Alert:** `JennGateDaemonOffline` (no heartbeat > 5 minutes)  
**Severity:** Critical  
**Estimated Impact:** Devices cannot accept gate connections; on-call access blocked

**Diagnosis Steps:**

1. **Identify which daemon(s) are offline:**
   - Go to Grafana → JennGate Phase 4 → "Daemon Health" table
   - Note daemon IDs with red highlighting (> 300 seconds)

2. **SSH to offline daemon:**
   ```bash
   # Example: daemon-123 is offline
   ssh -i /path/to/key ops@192.168.1.123  # Use daemon's IP
   ```

3. **Check daemon process status:**
   ```bash
   # Check if jenngate-daemon is running
   sudo systemctl status jenngate-daemon
   
   # View last 50 lines of daemon log
   journalctl -u jenngate-daemon -n 50 --no-pager
   
   # Check for crashes
   journalctl -u jenngate-daemon -p err
   ```

4. **Check network connectivity:**
   ```bash
   # From daemon, verify JennGate is reachable
   curl -k https://jenngate.internal:8081/health
   
   # Check DNS resolution
   nslookup jenngate.internal
   
   # Check firewall rules
   sudo ufw status | grep 8081
   ```

**Resolution Steps:**

1. **If daemon process not running:**
   ```bash
   # Restart daemon
   sudo systemctl restart jenngate-daemon
   
   # Monitor logs while restarting
   journalctl -u jenngate-daemon -f
   ```

2. **If process crashes immediately after restart:**
   - Check disk space: `df -h /`
   - Check memory: `free -h`
   - Check logs for crash reason: `journalctl -u jenngate-daemon -p crit --no-pager`
   - Contact daemon maintainer if issue unclear

3. **If network unreachable:**
   - Check IP connectivity: `ping jenngate.internal`
   - Check firewall: `sudo ufw status` (allow 8081)
   - Check DNS: `nslookup jenngate.internal`
   - If DNS broken, use IP directly in daemon config

4. **If certificate expired:**
   - Check device certificate: `openssl x509 -in /etc/jenngate/daemon.crt -noout -dates`
   - Regenerate if expired: Contact ops team
   - Restart daemon: `sudo systemctl restart jenngate-daemon`

5. **If heartbeat still not arriving after 5 minutes:**
   - Contact on-call engineer; may need JennGate restart or diagnostics

**Success Criteria:** Daemon appears in health table with heartbeat < 120 seconds

---

### Storage Quota High

**Alert:** `JennGateStorageQuotaHigh` (> 80% for 5min)  
**Severity:** Warning  
**Critical Alert:** `JennGateStorageQuotaCritical` (> 95% for 1min)

**Diagnosis Steps:**

1. **Check current storage usage:**
   ```bash
   # SSH to NAS host
   ssh -i /path/to/key ops@nas.internal
   
   # Check disk usage
   df -h /recordings
   
   # Find large directories
   du -sh /recordings/* | sort -rh | head -20
   ```

2. **Check recording archival process:**
   ```bash
   # Check if archival cron job is running
   crontab -l | grep archive
   
   # Check archival logs
   tail -50 /var/log/jenngate-archive.log
   ```

3. **Check Prometheus for recording upload rate:**
   ```promql
   # Recording upload rate (records/minute)
   rate(jenngate_recording_upload_duration_seconds_count[5m]) * 60
   
   # Average recording size (MB)
   avg(jenngate_recording_size_bytes) / 1024 / 1024
   ```

**Resolution Steps:**

1. **If < 90% full (warning alert):**
   - Plan storage expansion (1-2 weeks timeline)
   - Enable aggressive archival:
     ```bash
     # Archive recordings older than 7 days
     find /recordings -type f -mtime +7 -exec tar czf /archive/{}.tar.gz {} \;
     ```
   - Monitor disk usage daily

2. **If > 90% full (critical alert):**
   - **Immediate actions:**
     - Stop new recording uploads: Contact Jenn ops to disable recording_enabled flag
     - Archive all old recordings (> 30 days old):
       ```bash
       find /recordings -type f -mtime +30 -exec tar czf /archive/{}.tar.gz {} \; -delete
       ```
   - Check available archive capacity
   - If no archive space, delete oldest archived files
   - Re-enable recording after capacity restored

3. **If > 95% full (critical emergency):**
   - **Emergency cleanup:**
     ```bash
     # Delete all recordings older than 7 days (irreversible!)
     find /recordings -type f -mtime +7 -delete
     ```
   - Contact on-call storage engineer immediately
   - Plan emergency NAS expansion or decommission

**Success Criteria:** Usage drops below 80% and remains stable

---

### Troubleshooting Guide

#### Metrics not appearing in Grafana

**Symptoms:** Dashboard shows "No Data" for all panels

**Diagnosis:**
1. Verify Prometheus is scraping JennGate: `curl http://prometheus.internal:9090/api/v1/targets`
2. Check JennGate metrics endpoint: `curl http://jenngate.internal:9090/metrics | head -20`
3. Verify Prometheus config includes JennGate scrape job

**Resolution:**
1. If metrics endpoint not responding: Restart JennGate
2. If scrape job missing: Add to `prometheus.yml`:
   ```yaml
   scrape_configs:
     - job_name: jenngate
       static_configs:
         - targets: ['jenngate.internal:9090']
   ```
3. Reload Prometheus: `sudo systemctl reload prometheus`

#### Alert not firing but metric is high

**Symptoms:** Metric shows > threshold but alert not appearing

**Diagnosis:**
1. Check alert rule syntax: `curl http://prometheus.internal:9090/api/v1/rules | jq '.data.groups[] | select(.name=="jenngate_alerts")'`
2. Check alert is not silenced: Grafana → Alerting → Silences
3. Check notification channels configured: Grafana → Configuration → Notification channels

**Resolution:**
1. If rule syntax error: Fix `prometheus_rules.yaml` and reload
2. If silenced: Remove silence
3. If no channels: Add PagerDuty/Slack channel to notification policy

#### Dashboard showing stale data (timestamp old)

**Symptoms:** Dashboard data is hours old, not updating

**Diagnosis:**
1. Check Prometheus scrape status: `curl http://prometheus.internal:9090/api/v1/targets | jq '.data.activeTargets[0]'`
2. Check if JennGate is responding: `curl http://jenngate.internal:9090/metrics | head -1`

**Resolution:**
1. If target DOWN: Check JennGate health, restart if needed
2. If JennGate not responding: Restart JennGate service
3. If still stale: Check Prometheus disk space and restart

---

## Scaling Recommendations

Based on Phase 4 load test results (10 concurrent users, 50 certificates/minute):

### Estimated Capacity (single JennGate instance)

| Load | Duration | p99 Latency | CPU | Memory | Disk I/O |
|------|----------|-------------|-----|--------|----------|
| 10 users | baseline | 42ms cert | 15% | 128MB | low |
| 100 users | 10 min | 50-80ms cert | 40% | 256MB | medium |
| 1000 users | 1 min (burst) | 100-150ms cert | 80% | 512MB | high |
| 10000 users | unsupported | >1s cert | >95% | OOM | critical |

### Recommendations by Scale

**< 100 concurrent users:**
- Single JennGate instance sufficient
- Monitor CPU/memory, alert at 70%
- Recording archive weekly

**100-500 concurrent users:**
- Deploy 2x JennGate instances (load balanced)
- Add recording archival to daily schedule
- Storage expansion needed (100+ GB/month)

**500-2000 concurrent users:**
- Deploy 3x JennGate instances (2 active + 1 hot-standby)
- Implement read replicas for policy database
- NAS expansion or tiered storage (hot/cold)
- Archive recordings daily

**> 2000 concurrent users:**
- Custom architecture needed
- Recommend consulting with Jenn architecture team
- May require session multiplexing, policy caching, etc.

### Storage Growth Projections

Assuming 1 recording per session (not all sessions recorded):

| Recording Size | 100 users/hr | 1000 users/hr | 10000 users/hr |
|---|---|---|---|
| 500MB/hour (typical) | 50GB/day | 500GB/day | 5TB/day |
| 1GB/hour (verbose) | 100GB/day | 1TB/day | 10TB/day |

**Planning:** Budget 500GB+ NAS for production, with monthly growth at 50+ GB/month

---

## Operational Checklists

### Daily Monitoring Checklist

- [ ] Review Grafana dashboard for any yellow/red indicators
- [ ] Check daemon health table for offline devices
- [ ] Verify storage usage < 80%
- [ ] Review alert history: Any alerts fired in last 24h?
- [ ] Spot-check policy sync success rate (should be 99.9%+)

### Weekly Maintenance

- [ ] Review alert logs for patterns (recurring failures?)
- [ ] Check certificate pool for near-expiry certs (< 7 days)
- [ ] Test daemon heartbeat detection (kill daemon, verify alert)
- [ ] Review slow query logs in PostgreSQL
- [ ] Plan storage archival if needed

### Monthly Review

- [ ] Capacity planning: Will we exceed 80% storage in 30 days?
- [ ] Certificate rotation schedule review
- [ ] Archive old recordings (> 60 days)
- [ ] Performance trend analysis (latencies drifting up?)
- [ ] Upgrade planning for JennGate or dependencies

---

## References

- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Dashboard Documentation](https://grafana.com/docs/grafana/latest/dashboards/)
- [JennGate Architecture](./ARCHITECTURE.md)
- [Phase 4 Load Test Results](./LOAD_TEST_RESULTS.md)
- [Cutover Runbook](./PHASE4_RUNBOOK.md)

