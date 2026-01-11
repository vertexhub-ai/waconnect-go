package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/waconnect/waconnect-go/internal/core"
	"go.uber.org/zap"
)

// Session status constants
type SessionStatus string

const (
	StatusInitializing SessionStatus = "INITIALIZING"
	StatusConnecting   SessionStatus = "CONNECTING"
	StatusQRReady      SessionStatus = "QR_READY"
	StatusReady        SessionStatus = "READY"
	StatusDisconnected SessionStatus = "DISCONNECTED"
)

// Common errors
var (
	ErrSessionExists   = errors.New("session already exists")
	ErrSessionNotFound = errors.New("session not found")
	ErrNotConnected    = errors.New("not connected")
)

// WAClient represents a WhatsApp client session
type WAClient struct {
	ID               string
	status           SessionStatus
	phoneNumber      string
	qrCode           string
	qrCodeBase64     string
	connectedAt      *time.Time
	lastActivityAt   time.Time
	messagesSent     int
	messagesReceived int

	mu      sync.RWMutex
	logger  *zap.SugaredLogger
	dataDir string

	// Core connection
	conn      *core.Connection
	qrGen     *core.QRGenerator
	cancelCtx context.CancelFunc

	// Event handlers
	onQR      func(string)
	onReady   func()
	onMessage func(Message)
}

// Message represents a WhatsApp message
type Message struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	FromName  string    `json:"fromName"`
	To        string    `json:"to"`
	Text      string    `json:"text"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	IsFromMe  bool      `json:"isFromMe"`
}

// NewWAClient creates a new WhatsApp client
func NewWAClient(sessionID string, logger *zap.SugaredLogger, dataDir string) *WAClient {
	return &WAClient{
		ID:             sessionID,
		status:         StatusInitializing,
		lastActivityAt: time.Now(),
		logger:         logger,
		dataDir:        dataDir,
		qrGen:          core.NewQRGenerator(),
	}
}

// Connect establishes connection to WhatsApp
func (c *WAClient) Connect() error {
	c.mu.Lock()
	c.status = StatusConnecting
	c.mu.Unlock()

	c.logger.Infof("Connecting session %s...", c.ID)

	// Create core connection
	c.conn = core.NewConnection(core.ConnectionConfig{
		SessionID:           c.ID,
		SessionDir:          c.dataDir,
		ConnectTimeoutMs:    30000,
		KeepAliveIntervalMs: 30000,
		QRTimeoutMs:         60000,
		MaxRetries:          3,
		Logger:              c.logger,
	})

	// Set callbacks
	c.conn.SetOnQR(func(qrData string) {
		c.mu.Lock()
		c.status = StatusQRReady
		c.qrCode = qrData

		// Generate base64 image
		if base64, err := c.qrGen.GenerateBase64(qrData); err == nil {
			c.qrCodeBase64 = base64
		}
		c.lastActivityAt = time.Now()
		c.mu.Unlock()

		c.logger.Infof("QR Code ready for session %s", c.ID)

		if c.onQR != nil {
			c.onQR(qrData)
		}
	})

	c.conn.SetOnReady(func() {
		c.mu.Lock()
		now := time.Now()
		c.status = StatusReady
		c.connectedAt = &now
		c.lastActivityAt = now
		c.mu.Unlock()

		c.logger.Infof("Session %s connected!", c.ID)

		if c.onReady != nil {
			c.onReady()
		}
	})

	// Start connection in background
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelCtx = cancel

	go func() {
		if err := c.conn.Connect(ctx); err != nil {
			c.logger.Errorf("Connection failed for %s: %v", c.ID, err)
			c.mu.Lock()
			c.status = StatusDisconnected
			c.mu.Unlock()
		}
	}()

	// Wait for QR to be generated or connection to fail
	time.Sleep(3 * time.Second)

	return nil
}

// Disconnect closes the WhatsApp connection
func (c *WAClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = StatusDisconnected
	c.qrCode = ""
	c.logger.Infof("Session %s disconnected", c.ID)
}

// GetStatus returns current session status
func (c *WAClient) GetStatus() SessionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetQRCode returns the current QR code
func (c *WAClient) GetQRCode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.qrCode
}

// GetPhoneNumber returns the connected phone number
func (c *WAClient) GetPhoneNumber() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.phoneNumber
}

// GetSession returns session info
func (c *WAClient) GetSession() SessionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return SessionInfo{
		ID:               c.ID,
		Status:           c.status,
		PhoneNumber:      c.phoneNumber,
		ConnectedAt:      c.connectedAt,
		LastActivityAt:   c.lastActivityAt,
		MessagesSent:     c.messagesSent,
		MessagesReceived: c.messagesReceived,
	}
}

// SendText sends a text message
func (c *WAClient) SendText(to, text string) (*MessageResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusReady {
		return nil, ErrNotConnected
	}

	// TODO: Implement actual message sending
	c.messagesSent++
	c.lastActivityAt = time.Now()

	return &MessageResult{
		MessageID: "MSG_" + time.Now().Format("20060102150405"),
		Timestamp: time.Now(),
	}, nil
}

// SessionInfo holds session information
type SessionInfo struct {
	ID               string        `json:"id"`
	Status           SessionStatus `json:"status"`
	PhoneNumber      string        `json:"phoneNumber,omitempty"`
	ConnectedAt      *time.Time    `json:"connectedAt,omitempty"`
	LastActivityAt   time.Time     `json:"lastActivityAt"`
	MessagesSent     int           `json:"messagesSent"`
	MessagesReceived int           `json:"messagesReceived"`
}

// MessageResult holds the result of sending a message
type MessageResult struct {
	MessageID string    `json:"messageId"`
	Timestamp time.Time `json:"timestamp"`
}
