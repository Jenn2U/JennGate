package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/Jenn2U/JennGate/internal/models"
	"github.com/Jenn2U/JennGate/internal/services"
	"github.com/gin-gonic/gin"
)

// Handlers holds all service dependencies for HTTP handlers.
type Handlers struct {
	caService        *services.CAService
	sessionService   *services.SessionService
	recordingService *services.RecordingService
	db               *sql.DB
}

// NewHandlers creates a new Handlers instance with required services.
func NewHandlers(
	caService *services.CAService,
	sessionService *services.SessionService,
	recordingService *services.RecordingService,
	db *sql.DB,
) *Handlers {
	return &Handlers{
		caService:        caService,
		sessionService:   sessionService,
		recordingService: recordingService,
		db:               db,
	}
}

// RegisterRoutes registers all API endpoints with the Gin router.
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	// Health check
	router.GET("/health", h.Health)

	// API v1 endpoints (user-facing)
	api := router.Group("/api/v1/gate")
	{
		// Certificate issuance
		api.POST("/cert/issue", h.IssueCert)

		// Session management
		api.GET("/sessions", h.ListSessions)
		api.GET("/sessions/:session_id", h.GetSession)

		// Recording download
		api.GET("/recordings/:recording_id", h.GetRecording)
	}

	// Admin endpoints
	admin := router.Group("/admin/gate")
	{
		admin.GET("/pending-devices", h.ListPendingDevices)
		admin.POST("/devices/:device_id/approve", h.ApproveDevice)
		admin.POST("/devices/:device_id/decommission", h.DecommissionDevice)
	}

	// Daemon endpoints (internal)
	daemon := router.Group("/api/v1/gate")
	{
		daemon.POST("/sessions/:session_id/start", h.ReportSessionStart)
		daemon.POST("/sessions/:session_id/end", h.ReportSessionEnd)
	}

	// WebSocket terminal bridge
	router.GET("/ws/gate/:session_id", h.TerminalBridge)
}

// ============================================================================
// Health Check
// ============================================================================

// Health checks if JennGate is healthy.
// GET /health
func (h *Handlers) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify database connectivity
	dbStatus := "healthy"
	if err := h.db.PingContext(ctx); err != nil {
		dbStatus = "unhealthy: " + err.Error()
	}

	if dbStatus != "healthy" {
		c.JSON(http.StatusServiceUnavailable, models.HealthResponse{
			Status:   "unhealthy",
			Database: dbStatus,
		})
		return
	}

	c.JSON(http.StatusOK, models.HealthResponse{
		Status:   "healthy",
		Version:  "3.0.0", // TODO: Replace with VERSION from config
		Database: "healthy",
	})
}

// ============================================================================
// Certificate Endpoints
// ============================================================================

// IssueCert issues a new SSH certificate for a user to access a device.
// POST /api/v1/gate/cert/issue
func (h *Handlers) IssueCert(c *gin.Context) {
	var req models.IssueCertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	// Set default duration if not provided (1 hour)
	if req.DurationMinutes == 0 {
		req.DurationMinutes = 60
	}

	// TODO: Extract userID from JWT token (Phase 3b)
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "missing user_id",
		})
		return
	}

	// TODO: Verify user has permission to access device (Phase 3b)
	// TODO: Create session first (Phase 3b)
	// TODO: Generate user-specific private key (Phase 3b)

	// Generate certificate
	certBytes, err := h.caService.GenerateCertificate(userID, req.DeviceID, req.DurationMinutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed to generate certificate",
			Message: err.Error(),
		})
		return
	}

	// TODO: Return certificate and private key (Phase 3b - currently stub)
	expiresAt := time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute)

	c.JSON(http.StatusOK, models.IssueCertResponse{
		CertPEM:   string(certBytes),
		KeyPEM:    "", // TODO: Implement key generation (Phase 3b)
		ExpiresAt: expiresAt,
	})
}

// ============================================================================
// Session Endpoints
// ============================================================================

// ListSessions returns all sessions, optionally filtered by user/device/state.
// GET /api/v1/gate/sessions?user_id=...&device_id=...&state=...
func (h *Handlers) ListSessions(c *gin.Context) {
	var req models.ListSessionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid query parameters",
			Message: err.Error(),
		})
		return
	}

	// TODO: Implement filtering logic (Phase 3b)
	// For now, return empty list
	c.JSON(http.StatusOK, models.ListSessionsResponse{
		Sessions: []*models.SessionInfo{},
		Total:    0,
		Limit:    req.Limit,
		Offset:   req.Offset,
	})
}

// GetSession returns details of a single session.
// GET /api/v1/gate/sessions/:session_id
func (h *Handlers) GetSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	session, err := h.sessionService.GetSession(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "session not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed to get session",
			Message: err.Error(),
		})
		return
	}

	// Convert to response model
	certSerial := session.CertSerial
	certExpiresAt := session.CertExpiresAt
	startedAt := session.StartedAt
	info := &models.SessionInfo{
		ID:               session.ID,
		UserID:           session.UserID,
		DeviceID:         session.DeviceID,
		State:            session.State,
		CertSerial:       &certSerial,
		CertExpiresAt:    &certExpiresAt,
		StartedAt:        &startedAt,
		ConnectedAt:      session.ConnectedAt,
		DisconnectedAt:   session.DisconnectedAt,
		DisconnectReason: session.DisconnectReason,
		RecordingID:      session.RecordingID,
		CreatedAt:        session.CreatedAt,
	}

	c.JSON(http.StatusOK, info)
}

// ============================================================================
// Recording Endpoints
// ============================================================================

// GetRecording downloads a recording by ID.
// GET /api/v1/gate/recordings/:recording_id
func (h *Handlers) GetRecording(c *gin.Context) {
	recordingID := c.Param("recording_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	recording, err := h.recordingService.GetRecording(ctx, recordingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "recording not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "failed to get recording",
			Message: err.Error(),
		})
		return
	}

	// TODO: Verify user has permission to view this recording (Phase 3b)

	// Return recording as file download
	// TODO: Implement actual file streaming (Phase 3b)
	info := &models.RecordingInfo{
		ID:              recording.ID,
		SessionID:       recording.SessionID,
		UserID:          recording.UserID,
		DeviceID:        recording.DeviceID,
		FilePath:        recording.FilePath,
		ByteSize:        recording.ByteSize,
		DurationSeconds: recording.DurationSeconds,
		StartedAt:       recording.StartedAt,
		CompletedAt:     recording.CompletedAt,
		CreatedAt:       recording.CreatedAt,
	}

	c.JSON(http.StatusOK, info)
}

// ============================================================================
// Device Admin Endpoints
// ============================================================================

// ListPendingDevices returns devices awaiting approval.
// GET /admin/gate/pending-devices
func (h *Handlers) ListPendingDevices(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// TODO: Query devices with state=PENDING_APPROVAL (Phase 3b)
	_ = ctx

	c.JSON(http.StatusOK, models.ListPendingDevicesResponse{
		Devices: []*models.DeviceInfo{},
		Total:   0,
	})
}

// ApproveDevice approves a pending device for access.
// POST /admin/gate/devices/:device_id/approve
func (h *Handlers) ApproveDevice(c *gin.Context) {
	deviceID := c.Param("device_id")

	var req models.ApproveDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// TODO: Update device state to APPROVED (Phase 3b)
	// TODO: Sync policies to device (Phase 3b)
	// TODO: Log audit event (Phase 3b)
	_ = deviceID
	_ = req
	_ = ctx

	c.JSON(http.StatusOK, models.ApproveDeviceResponse{
		DeviceID:       deviceID,
		State:          "APPROVED",
		PoliciesSynced: false,
	})
}

// DecommissionDevice decommissions a device and removes all sessions/recordings.
// POST /admin/gate/devices/:device_id/decommission
func (h *Handlers) DecommissionDevice(c *gin.Context) {
	deviceID := c.Param("device_id")

	var req models.DecommissionDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// TODO: Terminate all sessions for device (Phase 3b)
	// TODO: Delete all recordings for device (Phase 3b)
	// TODO: Update device state to DECOMMISSIONED (Phase 3b)
	// TODO: Log audit event (Phase 3b)
	_ = deviceID
	_ = req
	_ = ctx

	c.JSON(http.StatusOK, models.DecommissionDeviceResponse{
		DeviceID:           deviceID,
		SessionsTerminated: 0,
		RecordingsDeleted:  0,
		AuditLogged:        false,
	})
}

// ============================================================================
// Daemon Endpoints (called by edge/dock daemons)
// ============================================================================

// ReportSessionStart is called by daemon when a session starts.
// POST /api/v1/gate/sessions/:session_id/start
func (h *Handlers) ReportSessionStart(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req models.ReportSessionStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// TODO: Verify daemon mTLS cert (Phase 3b)
	// TODO: Update session state to ACTIVE (Phase 3b)
	// TODO: Create recording for session (Phase 3b)
	_ = sessionID
	_ = req
	_ = ctx

	c.JSON(http.StatusOK, gin.H{
		"status": "recorded",
	})
}

// ReportSessionEnd is called by daemon when a session ends.
// POST /api/v1/gate/sessions/:session_id/end
func (h *Handlers) ReportSessionEnd(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req models.ReportSessionEndRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// TODO: Verify daemon mTLS cert (Phase 3b)
	// TODO: Mark session as DISCONNECTED with reason (Phase 3b)
	// TODO: Update recording with byte_size and duration (Phase 3b)
	_ = sessionID
	_ = req
	_ = ctx

	c.JSON(http.StatusOK, gin.H{
		"status": "acknowledged",
	})
}
