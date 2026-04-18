package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketUpgrader upgrades HTTP connections to WebSocket.
var WebSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking (Phase 3b)
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// TerminalMessage is a generic message structure for WebSocket communication.
type TerminalMessage struct {
	Type string `json:"type"` // "input", "output", "resize", "close"
	Data string `json:"data"` // Input text or output text
	Cols int    `json:"cols,omitempty"` // Terminal columns
	Rows int    `json:"rows,omitempty"` // Terminal rows
}

// TerminalSession holds WebSocket connection state for a single terminal session.
type TerminalSession struct {
	conn      *websocket.Conn
	sessionID string
	done      chan struct{}
	echo      bool // For Phase 3a: echo mode (no SSH connection)
}

// RegisterWebSocketRoutes registers WebSocket endpoints with the Gin router.
func (h *Handlers) RegisterWebSocketRoutes(router *gin.Engine) {
	router.GET("/ws/gate/:session_id", h.TerminalBridge)
}

// TerminalBridge is the WebSocket endpoint for interactive terminal access.
// GET /ws/gate/:session_id
func (h *Handlers) TerminalBridge(c *gin.Context) {
	sessionID := c.Param("session_id")

	// Upgrade HTTP connection to WebSocket
	conn, err := WebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed for session %s: %v", sessionID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "websocket upgrade failed",
		})
		return
	}

	// Create terminal session
	ts := &TerminalSession{
		conn:      conn,
		sessionID: sessionID,
		done:      make(chan struct{}),
		echo:      true, // Phase 3a: echo mode
	}

	// Handle the terminal session
	h.handleTerminalSession(ts)
}

// handleTerminalSession manages the lifecycle of a WebSocket terminal connection.
// Phase 3a: Echo stub implementation
// Phase 3b: Replace with actual SSH connection and forwarding
func (h *Handlers) handleTerminalSession(ts *TerminalSession) {
	defer ts.conn.Close()
	defer close(ts.done)

	// Set connection parameters
	ts.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	ts.conn.SetPongHandler(func(string) error {
		ts.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send welcome message (Phase 3a: echo stub)
	if err := ts.sendOutput("\r\nWelcome to JennGate Terminal (Phase 3a - Echo Mode)\r\n"); err != nil {
		log.Printf("Failed to send welcome message for session %s: %v", ts.sessionID, err)
		return
	}
	if err := ts.sendOutput("Type 'exit' to disconnect\r\n\r\n"); err != nil {
		log.Printf("Failed to send instructions for session %s: %v", ts.sessionID, err)
		return
	}

	// TODO: Connect to daemon SSH server (Phase 3b)
	// TODO: Forward bidirectional terminal I/O (Phase 3b)

	// Read messages from client
	for {
		var msg TerminalMessage
		err := ts.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for session %s: %v", ts.sessionID, err)
			}
			return
		}

		// Handle different message types
		switch msg.Type {
		case "input":
			h.handleTerminalInput(ts, msg)

		case "resize":
			// TODO: Resize PTY on daemon side (Phase 3b)
			h.handleTerminalResize(ts, msg)

		case "close":
			// Client requested close
			ts.sendOutput("\r\nDisconnecting...\r\n")
			return

		default:
			ts.sendOutput("Unknown message type: " + msg.Type + "\r\n")
		}
	}
}

// handleTerminalInput processes user input from the WebSocket client.
// Phase 3a: Echo mode - return input to client
// Phase 3b: Forward to SSH server
func (h *Handlers) handleTerminalInput(ts *TerminalSession, msg TerminalMessage) {
	if msg.Data == "" {
		return
	}

	// Phase 3a: Echo mode
	if ts.echo {
		// Check for exit command
		if msg.Data == "exit\r" || msg.Data == "exit\n" {
			ts.sendOutput("\r\nGoodbye!\r\n")
			ts.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		// Echo back the input
		ts.sendOutput(msg.Data)

		// If it was a newline, send a prompt
		if msg.Data == "\r" || msg.Data == "\n" || msg.Data == "\r\n" {
			ts.sendOutput("$ ")
		}
		return
	}

	// TODO: Forward to SSH connection (Phase 3b)
	// For now, in non-echo mode, just acknowledge
	ts.sendOutput("(SSH mode not yet implemented)\r\n")
}

// handleTerminalResize handles terminal resize messages from the client.
// Phase 3a: Log the resize request
// Phase 3b: Resize PTY on daemon side
func (h *Handlers) handleTerminalResize(ts *TerminalSession, msg TerminalMessage) {
	if msg.Cols == 0 || msg.Rows == 0 {
		log.Printf("Invalid resize dimensions for session %s: cols=%d rows=%d", ts.sessionID, msg.Cols, msg.Rows)
		return
	}

	log.Printf("Terminal resize for session %s: %dx%d", ts.sessionID, msg.Cols, msg.Rows)

	// TODO: Send resize command to daemon (Phase 3b)
	// TIOCSWINSZ ioctl on daemon's PTY
}

// sendOutput sends output data to the WebSocket client.
func (ts *TerminalSession) sendOutput(data string) error {
	msg := TerminalMessage{
		Type: "output",
		Data: data,
	}
	ts.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ts.conn.WriteJSON(msg)
}

// ===================================================================
// WebSocket Terminal Utilities (Phase 3a)
// ===================================================================

// sendMessage sends a generic message to the WebSocket client.
func (ts *TerminalSession) sendMessage(msgType string, data string) error {
	msg := TerminalMessage{
		Type: msgType,
		Data: data,
	}
	ts.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ts.conn.WriteJSON(msg)
}
