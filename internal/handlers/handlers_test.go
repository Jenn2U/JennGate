package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Jenn2U/JennGate/internal/models"
	"github.com/Jenn2U/JennGate/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHandlers creates a test Handlers instance with mocked services.
func setupHandlers(t *testing.T) *Handlers {
	// For tests, we're using real service instances but they may not have all dependencies
	// Tests should mock responses where necessary
	caService := &services.CAService{}
	sessionService := &services.SessionService{}
	recordingService := &services.RecordingService{}
	policyService := services.NewPolicyService()

	return NewHandlers(caService, sessionService, recordingService, policyService, nil)
}

// setupTestRouter creates a router with user_id middleware for testing
func setupTestRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add middleware to set user_id in context
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})

	h.RegisterRoutes(router)
	return router
}

// setupRouter creates a Gin router with registered routes.
func setupRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router)
	return router
}

// TestIssueCertRequestParsing tests that IssueCertRequest correctly parses JSON with enable_gui field.
func TestIssueCertRequestParsing(t *testing.T) {
	var req models.IssueCertRequest

	// Test with enable_gui = false
	body := []byte(`{"device_id": "device-123", "duration_minutes": 60, "enable_gui": false}`)
	err := json.Unmarshal(body, &req)
	require.NoError(t, err)
	assert.Equal(t, "device-123", req.DeviceID)
	assert.Equal(t, 60, req.DurationMinutes)
	assert.False(t, req.EnableGUI)

	// Test with enable_gui = true
	body = []byte(`{"device_id": "device-456", "duration_minutes": 120, "enable_gui": true}`)
	err = json.Unmarshal(body, &req)
	require.NoError(t, err)
	assert.Equal(t, "device-456", req.DeviceID)
	assert.Equal(t, 120, req.DurationMinutes)
	assert.True(t, req.EnableGUI)

	// Test without enable_gui (boolean fields default to false in Go)
	body = []byte(`{"device_id": "device-789", "duration_minutes": 45}`)
	req = models.IssueCertRequest{} // Reset
	err = json.Unmarshal(body, &req)
	require.NoError(t, err)
	assert.Equal(t, "device-789", req.DeviceID)
	assert.Equal(t, 45, req.DurationMinutes)
	// EnableGUI will be false by default (zero value for bool)
	assert.Equal(t, false, req.EnableGUI)
}

// TestIssueCertResponseStructure tests that IssueCertResponse includes GUI fields.
func TestIssueCertResponseStructure(t *testing.T) {
	// Test response with no GUI
	resp := models.IssueCertResponse{
		CertPEM:      "cert-data",
		KeyPEM:       "key-data",
		ExpiresAt:    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		SessionID:    "session-123",
		SSHPort:      2222,
		GUIAvailable: false,
		VNCPort:      nil,
		X11Display:   nil,
	}

	assert.NotEmpty(t, resp.CertPEM)
	assert.NotEmpty(t, resp.SessionID)
	assert.Equal(t, 2222, resp.SSHPort)
	assert.False(t, resp.GUIAvailable)
	assert.Nil(t, resp.VNCPort)
	assert.Nil(t, resp.X11Display)

	// Test response with GUI
	vncPort := 5900
	resp2 := models.IssueCertResponse{
		CertPEM:      "cert-data",
		KeyPEM:       "key-data",
		ExpiresAt:    time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		SessionID:    "session-456",
		SSHPort:      2222,
		GUIAvailable: true,
		VNCPort:      &vncPort,
		X11Display:   nil,
	}

	assert.True(t, resp2.GUIAvailable)
	assert.NotNil(t, resp2.VNCPort)
	assert.Equal(t, 5900, *resp2.VNCPort)
}

// TestSessionStatusResponseStructure tests that SessionStatusResponse includes GUI fields.
func TestSessionStatusResponseStructure(t *testing.T) {
	// Test session status with no GUI
	resp := models.SessionStatusResponse{
		SessionID:   "session-123",
		State:       "ACTIVE",
		SSHActive:   true,
		GUIActive:   false,
		GUIProtocol: nil,
		GUIPort:     nil,
		SSHPort:     2222,
		StartedAt:   time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}

	assert.Equal(t, "session-123", resp.SessionID)
	assert.Equal(t, "ACTIVE", resp.State)
	assert.True(t, resp.SSHActive)
	assert.False(t, resp.GUIActive)
	assert.Nil(t, resp.GUIProtocol)
	assert.Nil(t, resp.GUIPort)
	assert.Equal(t, 2222, resp.SSHPort)

	// Test session status with GUI
	guiProtocol := "vnc"
	guiPort := 5900
	resp2 := models.SessionStatusResponse{
		SessionID:   "session-456",
		State:       "ACTIVE",
		SSHActive:   true,
		GUIActive:   true,
		GUIProtocol: &guiProtocol,
		GUIPort:     &guiPort,
		SSHPort:     2222,
		StartedAt:   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}

	assert.True(t, resp2.GUIActive)
	assert.NotNil(t, resp2.GUIProtocol)
	assert.Equal(t, "vnc", *resp2.GUIProtocol)
	assert.NotNil(t, resp2.GUIPort)
	assert.Equal(t, 5900, *resp2.GUIPort)
}

// TestSessionStatusEndpointRegistration verifies that the status endpoint is registered.
// This test just verifies the route registration, not the full implementation.
func TestSessionStatusEndpointRegistration(t *testing.T) {
	// Create a simple router and register routes to verify endpoint exists
	gin.SetMode(gin.TestMode)
	router := gin.New()

	h := setupHandlers(t)
	h.RegisterRoutes(router)

	// Test that the endpoint route is registered by checking its existence
	// The route should match /api/v1/gate/sessions/:session_id/status
	routes := router.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/v1/gate/sessions/:session_id/status" && route.Method == "GET" {
			found = true
			break
		}
	}

	assert.True(t, found, "SessionStatus endpoint should be registered at GET /api/v1/gate/sessions/:session_id/status")
}


// TestIssueCertResponseFormatRFC3339 tests that ExpiresAt uses RFC3339 format string.
func TestIssueCertResponseFormatRFC3339(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	resp := models.IssueCertResponse{
		CertPEM:      "cert-data",
		KeyPEM:       "key-data",
		ExpiresAt:    expiresAt,
		SessionID:    "session-123",
		SSHPort:      2222,
		GUIAvailable: false,
	}

	// Verify ExpiresAt is a valid RFC3339 timestamp string
	parsedTime, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	assert.NoError(t, err, "ExpiresAt should be a valid RFC3339 timestamp")
	assert.True(t, parsedTime.After(time.Now()))
}
