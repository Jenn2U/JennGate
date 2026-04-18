package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"

	"github.com/Jenn2U/JennGate/internal/services"
	"google.golang.org/grpc"
)

// DaemonServer implements the gRPC daemon service for edge/dock devices.
// Daemons connect to register themselves, report session events, and fetch policies.
type DaemonServer struct {
	sessionService   *services.SessionService
	recordingService *services.RecordingService
	db               *sql.DB
}

// NewDaemonServer creates a new gRPC daemon server.
func NewDaemonServer(
	sessionService *services.SessionService,
	recordingService *services.RecordingService,
	db *sql.DB,
) *DaemonServer {
	return &DaemonServer{
		sessionService:   sessionService,
		recordingService: recordingService,
		db:               db,
	}
}

// StartDaemonServer starts the gRPC daemon server on the given port.
func StartDaemonServer(port int, daemonServer *DaemonServer) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	server := grpc.NewServer(
		// TODO: Add mTLS credentials (Phase 3b)
		// TODO: Add authentication interceptor (Phase 3b)
	)

	// Register daemon service
	// TODO: Register proto service here (Phase 3b)
	// For Phase 3a, we'll register method handlers manually

	log.Printf("Starting gRPC daemon server on port %d", port)
	if err := server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

// ===================================================================
// Daemon RPC Methods (Phase 3a: Stub implementations)
// ===================================================================

// RegisterDaemon is called when a device daemon starts and registers itself.
// The daemon sends its device_id, type, version, and public key for mTLS.
// JennGate responds with the current device state (PENDING_APPROVAL or APPROVED)
// and syncs policies if approved.
//
// Phase 3a: Basic implementation that records device as PENDING_APPROVAL
// Phase 3b: Full implementation with policy sync, mTLS validation, etc.
func (ds *DaemonServer) RegisterDaemon(
	ctx context.Context,
	deviceID string,
	deviceType string,
	daemonVersion string,
	publicKeyPEM string,
) (string, error) {
	log.Printf("RegisterDaemon called: deviceID=%s type=%s version=%s", deviceID, deviceType, daemonVersion)

	// TODO: Store daemon public key in DB for mTLS (Phase 3b)
	// TODO: Update device state and metadata (Phase 3b)
	// TODO: Sync access policies to device (Phase 3b)

	// Phase 3a: Stub response - device is pending approval
	state := "PENDING_APPROVAL"
	log.Printf("Device %s registered with state: %s", deviceID, state)

	return state, nil
}

// ReportSessionStart is called when a daemon starts an SSH session.
// The daemon reports the session_id and SSH port for the session.
// JennGate records the session as ACTIVE and prepares recording capture.
//
// Phase 3a: Log the session start
// Phase 3b: Full implementation with recording initialization, etc.
func (ds *DaemonServer) ReportSessionStart(
	ctx context.Context,
	sessionID string,
	sshPort int,
) error {
	log.Printf("ReportSessionStart called: sessionID=%s sshPort=%d", sessionID, sshPort)

	// TODO: Verify session exists and is in AUTHORIZED state (Phase 3b)
	// TODO: Create recording entry (Phase 3b)
	// TODO: Update session state to ACTIVE (Phase 3b)
	// TODO: Log audit event (Phase 3b)

	return nil
}

// ReportSessionEnd is called when a daemon ends an SSH session.
// The daemon reports the session_id and disconnect reason.
// JennGate marks the session as DISCONNECTED and finalizes the recording.
//
// Phase 3a: Log the session end
// Phase 3b: Full implementation with recording finalization, etc.
func (ds *DaemonServer) ReportSessionEnd(
	ctx context.Context,
	sessionID string,
	reason string,
) error {
	log.Printf("ReportSessionEnd called: sessionID=%s reason=%s", sessionID, reason)

	// TODO: Verify session exists (Phase 3b)
	// TODO: Mark session as DISCONNECTED (Phase 3b)
	// TODO: Finalize recording (byte size, duration) (Phase 3b)
	// TODO: Log audit event (Phase 3b)

	return nil
}

// FetchPolicies is called by daemons to fetch access control policies.
// Used both on initial registration (if approved) and periodically.
// JennGate returns the list of policies that apply to this device.
// Policies are compressed and cached for offline evaluation on the daemon.
//
// Phase 3a: Return empty policy list
// Phase 3b: Full implementation with policy fetching, CRDT sync, etc.
func (ds *DaemonServer) FetchPolicies(
	ctx context.Context,
	deviceID string,
) ([]map[string]interface{}, error) {
	log.Printf("FetchPolicies called: deviceID=%s", deviceID)

	// TODO: Query access policies for device from DB (Phase 3b)
	// TODO: Serialize policies (JSON or protobuf) (Phase 3b)
	// TODO: Return with sync token for CRDT (Phase 3b)

	// Phase 3a: Return empty list
	return []map[string]interface{}{}, nil
}

// ===================================================================
// Daemon Helper Functions
// ===================================================================

// ValidateDaemonMTLS validates the mTLS certificate from a daemon.
// Phase 3a: Stub (always passes)
// Phase 3b: Full implementation with cert validation
func (ds *DaemonServer) ValidateDaemonMTLS(ctx context.Context, deviceID string) error {
	// TODO: Validate mTLS cert against stored public key (Phase 3b)
	return nil
}

// ApplyPolicyUpdatesToDaemon sends updated policies to a daemon.
// Called by policy sync system when policies change.
// Phase 3a: Stub
// Phase 3b: Full implementation with gRPC push
func (ds *DaemonServer) ApplyPolicyUpdatesToDaemon(
	ctx context.Context,
	deviceID string,
	policies []map[string]interface{},
) error {
	log.Printf("ApplyPolicyUpdatesToDaemon: deviceID=%s policies=%d", deviceID, len(policies))
	// TODO: Send policies to daemon via gRPC (Phase 3b)
	return nil
}

// NotifyDaemonSessionTermination sends a kill signal for a session to a daemon.
// Called when decommissioning a device or force-terminating a session.
// Phase 3a: Stub
// Phase 3b: Full implementation
func (ds *DaemonServer) NotifyDaemonSessionTermination(
	ctx context.Context,
	deviceID string,
	sessionID string,
) error {
	log.Printf("NotifyDaemonSessionTermination: deviceID=%s sessionID=%s", deviceID, sessionID)
	// TODO: Send termination signal to daemon (Phase 3b)
	return nil
}

// ===================================================================
// gRPC Service (Phase 3b: Full proto-based implementation)
// ===================================================================

// NOTE: For Phase 3a, the above methods are stubs.
// In Phase 3b, these will be registered with a gRPC server generated from:
//
// service JennGateDaemon {
//   rpc RegisterDaemon(RegisterDaemonRequest) returns (RegisterDaemonResponse);
//   rpc ReportSessionStart(SessionStartRequest) returns (Empty);
//   rpc ReportSessionEnd(SessionEndRequest) returns (Empty);
//   rpc FetchPolicies(FetchPoliciesRequest) returns (AccessPoliciesResponse);
// }
//
// message RegisterDaemonRequest {
//   string device_id = 1;
//   string device_type = 2;
//   string daemon_version = 3;
//   string public_key_pem = 4;
// }
//
// message RegisterDaemonResponse {
//   enum State {
//     PENDING_APPROVAL = 0;
//     APPROVED = 1;
//   }
//   State state = 1;
//   repeated AccessPolicy policies = 2;
// }
//
// message SessionStartRequest {
//   string session_id = 1;
//   int32 ssh_port = 2;
// }
//
// message SessionEndRequest {
//   string session_id = 1;
//   string reason = 2;
// }
//
// message FetchPoliciesRequest {
//   string device_id = 1;
// }
//
// message AccessPolicy {
//   string principal_type = 1;
//   string principal_id = 2;
//   repeated string permissions = 3;
// }
//
// message AccessPoliciesResponse {
//   repeated AccessPolicy policies = 1;
// }
