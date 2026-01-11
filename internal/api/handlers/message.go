package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/waconnect/waconnect-go/internal/client"
	"go.uber.org/zap"
)

// MessageHandler handles message-related requests
type MessageHandler struct {
	sessionManager *client.SessionManager
	logger         *zap.SugaredLogger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(sm *client.SessionManager, logger *zap.SugaredLogger) *MessageHandler {
	return &MessageHandler{
		sessionManager: sm,
		logger:         logger,
	}
}

// SendTextRequest represents a text message request
type SendTextRequest struct {
	SessionID string `json:"sessionId"`
	To        string `json:"to"`
	Text      string `json:"text"`
}

// SendText sends a text message
func (h *MessageHandler) SendText(c *fiber.Ctx) error {
	var req SendTextRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if req.SessionID == "" || req.To == "" || req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "sessionId, to, and text are required",
		})
	}

	// Get session
	session, exists := h.sessionManager.GetSession(req.SessionID)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Session not found",
		})
	}

	// Check session is ready
	if session.GetStatus() != client.StatusReady {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Session not connected",
		})
	}

	// Send message
	result, err := session.SendText(req.To, req.Text)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// SendMediaRequest represents a media message request
type SendMediaRequest struct {
	SessionID string `json:"sessionId"`
	To        string `json:"to"`
	MediaURL  string `json:"mediaUrl"`
	Caption   string `json:"caption"`
	Type      string `json:"type"` // image, video, audio, document
}

// SendMedia sends a media message
func (h *MessageHandler) SendMedia(c *fiber.Ctx) error {
	var req SendMediaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if req.SessionID == "" || req.To == "" || req.MediaURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "sessionId, to, and mediaUrl are required",
		})
	}

	// TODO: Implement media sending
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messageId": "MEDIA_PLACEHOLDER",
			"status":    "sent",
		},
	})
}

// SendLocationRequest represents a location message request
type SendLocationRequest struct {
	SessionID string  `json:"sessionId"`
	To        string  `json:"to"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}

// SendLocation sends a location message
func (h *MessageHandler) SendLocation(c *fiber.Ctx) error {
	var req SendLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if req.SessionID == "" || req.To == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "sessionId and to are required",
		})
	}

	// TODO: Implement location sending
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messageId": "LOCATION_PLACEHOLDER",
			"status":    "sent",
		},
	})
}
