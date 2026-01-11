package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/waconnect/waconnect-go/internal/client"
	"go.uber.org/zap"
)

// SessionHandler handles session-related requests
type SessionHandler struct {
	sessionManager *client.SessionManager
	logger         *zap.SugaredLogger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sm *client.SessionManager, logger *zap.SugaredLogger) *SessionHandler {
	return &SessionHandler{
		sessionManager: sm,
		logger:         logger,
	}
}

// CreateRequest represents session creation request
type CreateRequest struct {
	SessionID string `json:"sessionId"`
}

// Create handles session creation
func (h *SessionHandler) Create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Generate session ID if not provided
	if req.SessionID == "" {
		req.SessionID = generateSessionID()
	}

	// Create session
	session, err := h.sessionManager.CreateSession(req.SessionID)
	if err != nil {
		if err == client.ErrSessionExists {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   "Session already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    session.GetSession(),
	})
}

// List returns all sessions
func (h *SessionHandler) List(c *fiber.Ctx) error {
	sessions := h.sessionManager.GetAllSessions()
	
	sessionInfos := make([]client.SessionInfo, len(sessions))
	for i, s := range sessions {
		sessionInfos[i] = s.GetSession()
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"sessions": sessionInfos,
			"stats":    h.sessionManager.GetStats(),
		},
	})
}

// Get returns a specific session
func (h *SessionHandler) Get(c *fiber.Ctx) error {
	sessionID := c.Params("id")

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Session not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    session.GetSession(),
	})
}

// GetQR returns the QR code for a session
func (h *SessionHandler) GetQR(c *fiber.Ctx) error {
	sessionID := c.Params("id")

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Session not found",
		})
	}

	qrCode := session.GetQRCode()
	if qrCode == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "QR code not available",
		})
	}

	// TODO: Generate actual SVG QR code
	// For now return the data
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"qr":     qrCode,
			"status": session.GetStatus(),
		},
	})
}

// GetStatus returns session status
func (h *SessionHandler) GetStatus(c *fiber.Ctx) error {
	sessionID := c.Params("id")

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Session not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"status":      session.GetStatus(),
			"phoneNumber": session.GetPhoneNumber(),
		},
	})
}

// Delete removes a session
func (h *SessionHandler) Delete(c *fiber.Ctx) error {
	sessionID := c.Params("id")

	err := h.sessionManager.DeleteSession(sessionID)
	if err != nil {
		if err == client.ErrSessionNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Session not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Session deleted",
	})
}

func generateSessionID() string {
	return "session-" + time.Now().Format("20060102150405")
}
