package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/waconnect/waconnect-go/internal/api"
	"github.com/waconnect/waconnect-go/internal/client"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	sugar.Info("🚀 WAConnect Go starting...")

	// Get config from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "3200"
	}

	// Initialize session manager
	sessionManager := client.NewSessionManager(sugar)

	// Load persisted sessions
	if err := sessionManager.LoadPersistedSessions(); err != nil {
		sugar.Warnf("Failed to load persisted sessions: %v", err)
	}

	// Initialize API server
	server := api.NewServer(api.ServerConfig{
		Port:           port,
		Logger:         sugar,
		SessionManager: sessionManager,
	})

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			sugar.Fatalf("Server failed: %v", err)
		}
	}()

	sugar.Infof("✅ WAConnect Go running at http://0.0.0.0:%s", port)
	sugar.Info("📚 API Docs available at /docs")
	sugar.Info("📱 Dashboard available at /dashboard")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugar.Info("Shutting down gracefully...")
	sessionManager.DisconnectAll()
	server.Stop()
}
