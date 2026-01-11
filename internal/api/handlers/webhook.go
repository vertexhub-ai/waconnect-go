package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/waconnect/waconnect-go/internal/webhook"
	"go.uber.org/zap"
)

// WebhookHandler handles webhook-related requests
type WebhookHandler struct {
	dispatcher *webhook.Dispatcher
	logger     *zap.SugaredLogger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(dispatcher *webhook.Dispatcher, logger *zap.SugaredLogger) *WebhookHandler {
	return &WebhookHandler{
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// CreateRequest represents webhook creation request
type WebhookCreateRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

// Create handles webhook creation
func (h *WebhookHandler) Create(c *fiber.Ctx) error {
	var req WebhookCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate URL
	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "URL is required",
		})
	}

	// Default to all events if none specified
	if len(req.Events) == 0 {
		req.Events = []string{"*"}
	}

	wh, err := h.dispatcher.Register(req.URL, req.Events, req.Secret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    wh,
	})
}

// List returns all webhooks
func (h *WebhookHandler) List(c *fiber.Ctx) error {
	webhooks := h.dispatcher.List()

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"webhooks": webhooks,
			"total":    len(webhooks),
		},
	})
}

// Delete removes a webhook
func (h *WebhookHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	err := h.dispatcher.Unregister(id)
	if err != nil {
		if err == webhook.ErrWebhookNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Webhook not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Webhook deleted",
	})
}

// Test sends a test event to a webhook
func (h *WebhookHandler) Test(c *fiber.Ctx) error {
	id := c.Params("id")

	// Dispatch test event
	h.dispatcher.Dispatch("webhook.test", fiber.Map{
		"webhookId": id,
		"message":   "This is a test event from WAConnect Go",
		"timestamp": c.Context().Time().Format("2006-01-02T15:04:05Z07:00"),
	})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Test event dispatched",
	})
}

// AvailableEvents returns list of available event types
func (h *WebhookHandler) AvailableEvents(c *fiber.Ctx) error {
	events := []fiber.Map{
		{"type": "session.connected", "description": "Fired when session connects successfully"},
		{"type": "session.disconnected", "description": "Fired when session disconnects"},
		{"type": "session.qr_ready", "description": "Fired when QR code is ready to scan"},
		{"type": "message.received", "description": "Fired when a message is received"},
		{"type": "message.sent", "description": "Fired when a message is sent"},
		{"type": "message.delivered", "description": "Fired when a message is delivered"},
		{"type": "message.read", "description": "Fired when a message is read"},
		{"type": "*", "description": "Subscribe to all events"},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    events,
	})
}
