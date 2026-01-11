package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/waconnect/waconnect-go/internal/api/handlers"
	"github.com/waconnect/waconnect-go/internal/api/middleware"
	"github.com/waconnect/waconnect-go/internal/client"
	"github.com/waconnect/waconnect-go/internal/webhook"
	"go.uber.org/zap"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Port           string
	Logger         *zap.SugaredLogger
	SessionManager *client.SessionManager
}

// Server represents the API server
type Server struct {
	app               *fiber.App
	config            ServerConfig
	sessionHandler    *handlers.SessionHandler
	messageHandler    *handlers.MessageHandler
	webhookHandler    *handlers.WebhookHandler
	webhookDispatcher *webhook.Dispatcher
}

// NewServer creates a new API server
func NewServer(config ServerConfig) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "WAConnect Go",
		ServerHeader: "WAConnect",
		ErrorHandler: customErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, X-API-Key, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Create webhook dispatcher
	webhookDispatcher := webhook.NewDispatcher(config.Logger)

	// Create handlers
	sessionHandler := handlers.NewSessionHandler(config.SessionManager, config.Logger)
	messageHandler := handlers.NewMessageHandler(config.SessionManager, config.Logger)
	webhookHandler := handlers.NewWebhookHandler(webhookDispatcher, config.Logger)

	server := &Server{
		app:               app,
		config:            config,
		sessionHandler:    sessionHandler,
		messageHandler:    messageHandler,
		webhookHandler:    webhookHandler,
		webhookDispatcher: webhookDispatcher,
	}

	server.setupRoutes()

	return server
}

// GetWebhookDispatcher returns the webhook dispatcher for event dispatch
func (s *Server) GetWebhookDispatcher() *webhook.Dispatcher {
	return s.webhookDispatcher
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check (no auth required)
	s.app.Get("/health", s.healthHandler)

	// Redirect root to dashboard
	s.app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/dashboard")
	})

	// Serve static files for dashboard
	s.app.Static("/dashboard", "./public")

	// API v1 routes with authentication
	api := s.app.Group("/api/v1", middleware.APIKeyAuth())

	// Session routes
	session := api.Group("/session")
	session.Post("/create", s.sessionHandler.Create)
	session.Get("/", s.sessionHandler.List)
	session.Get("/:id", s.sessionHandler.Get)
	session.Get("/:id/qr", s.sessionHandler.GetQR)
	session.Get("/:id/status", s.sessionHandler.GetStatus)
	session.Delete("/:id", s.sessionHandler.Delete)

	// Message routes
	send := api.Group("/send")
	send.Post("/text", s.messageHandler.SendText)
	send.Post("/media", s.messageHandler.SendMedia)
	send.Post("/location", s.messageHandler.SendLocation)

	// Webhook routes (n8n-ready)
	webhooks := api.Group("/webhooks")
	webhooks.Get("/", s.webhookHandler.List)
	webhooks.Post("/", s.webhookHandler.Create)
	webhooks.Delete("/:id", s.webhookHandler.Delete)
	webhooks.Post("/:id/test", s.webhookHandler.Test)
	webhooks.Get("/events", s.webhookHandler.AvailableEvents)

	// OpenAPI spec
	api.Get("/openapi.json", s.openAPISpec)
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *fiber.Ctx) error {
	stats := s.config.SessionManager.GetStats()
	return c.JSON(fiber.Map{
		"status":   "ok",
		"version":  "1.0.0",
		"sessions": stats,
	})
}

func (s *Server) openAPISpec(c *fiber.Ctx) error {
	// TODO: Generate proper OpenAPI spec
	return c.JSON(fiber.Map{
		"openapi": "3.0.0",
		"info": fiber.Map{
			"title":   "WAConnect Go API",
			"version": "1.0.0",
		},
	})
}

// Start starts the server
func (s *Server) Start() error {
	return s.app.Listen(fmt.Sprintf(":%s", s.config.Port))
}

// Stop stops the server
func (s *Server) Stop() error {
	return s.app.Shutdown()
}

// Custom error handler
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error":   err.Error(),
	})
}
