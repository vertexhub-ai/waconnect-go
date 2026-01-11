package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Webhook represents a registered webhook
type Webhook struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"secret,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

// Event represents a webhook event
type Event struct {
	Type      string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	WebhookID string      `json:"webhookId,omitempty"`
	Signature string      `json:"signature,omitempty"`
	Data      interface{} `json:"data"`
}

// Common event types
const (
	EventSessionConnected    = "session.connected"
	EventSessionDisconnected = "session.disconnected"
	EventSessionQRReady      = "session.qr_ready"
	EventMessageReceived     = "message.received"
	EventMessageSent         = "message.sent"
	EventMessageDelivered    = "message.delivered"
	EventMessageRead         = "message.read"
)

// Dispatcher handles webhook dispatch
type Dispatcher struct {
	webhooks   map[string]*Webhook
	mu         sync.RWMutex
	logger     *zap.SugaredLogger
	httpClient *http.Client
	maxRetries int
}

// NewDispatcher creates a new webhook dispatcher
func NewDispatcher(logger *zap.SugaredLogger) *Dispatcher {
	return &Dispatcher{
		webhooks: make(map[string]*Webhook),
		logger:   logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxRetries: 3,
	}
}

// Register registers a new webhook
func (d *Dispatcher) Register(url string, events []string, secret string) (*Webhook, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	webhook := &Webhook{
		ID:        "wh_" + uuid.New().String()[:8],
		URL:       url,
		Events:    events,
		Secret:    secret,
		Active:    true,
		CreatedAt: time.Now(),
	}

	d.webhooks[webhook.ID] = webhook
	d.logger.Infof("Registered webhook %s for events %v", webhook.ID, events)

	return webhook, nil
}

// Unregister removes a webhook
func (d *Dispatcher) Unregister(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.webhooks[id]; !exists {
		return ErrWebhookNotFound
	}

	delete(d.webhooks, id)
	d.logger.Infof("Unregistered webhook %s", id)

	return nil
}

// List returns all registered webhooks
func (d *Dispatcher) List() []*Webhook {
	d.mu.RLock()
	defer d.mu.RUnlock()

	webhooks := make([]*Webhook, 0, len(d.webhooks))
	for _, wh := range d.webhooks {
		// Hide secret in list
		whCopy := *wh
		if whCopy.Secret != "" {
			whCopy.Secret = "***"
		}
		webhooks = append(webhooks, &whCopy)
	}

	return webhooks
}

// Dispatch sends an event to all matching webhooks
func (d *Dispatcher) Dispatch(eventType string, data interface{}) {
	d.mu.RLock()
	matchingWebhooks := make([]*Webhook, 0)

	for _, wh := range d.webhooks {
		if !wh.Active {
			continue
		}

		// Check if webhook is subscribed to this event
		for _, event := range wh.Events {
			if event == eventType || event == "*" {
				matchingWebhooks = append(matchingWebhooks, wh)
				break
			}
		}
	}
	d.mu.RUnlock()

	// Dispatch to each matching webhook in parallel
	for _, wh := range matchingWebhooks {
		go d.sendWebhook(wh, eventType, data)
	}
}

// sendWebhook sends an event to a webhook with retries
func (d *Dispatcher) sendWebhook(wh *Webhook, eventType string, data interface{}) {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
		WebhookID: wh.ID,
		Data:      data,
	}

	// Generate signature if secret is set
	if wh.Secret != "" {
		event.Signature = d.generateSignature(event, wh.Secret)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		d.logger.Errorf("Failed to marshal webhook payload: %v", err)
		return
	}

	// Retry with exponential backoff
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("POST", wh.URL, bytes.NewBuffer(payload))
		if err != nil {
			d.logger.Errorf("Failed to create webhook request: %v", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-ID", wh.ID)
		req.Header.Set("X-Webhook-Event", eventType)
		if event.Signature != "" {
			req.Header.Set("X-Webhook-Signature", event.Signature)
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			d.logger.Warnf("Webhook delivery failed (attempt %d): %v", attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			d.logger.Debugf("Webhook delivered: %s -> %s", eventType, wh.URL)
			return
		}

		d.logger.Warnf("Webhook returned %d (attempt %d)", resp.StatusCode, attempt+1)
	}

	d.logger.Errorf("Failed to deliver webhook after %d attempts: %s", d.maxRetries+1, wh.URL)
}

// generateSignature creates HMAC-SHA256 signature
func (d *Dispatcher) generateSignature(event Event, secret string) string {
	payload, _ := json.Marshal(event.Data)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// Error types
var ErrWebhookNotFound = &WebhookError{Message: "webhook not found"}

type WebhookError struct {
	Message string
}

func (e *WebhookError) Error() string {
	return e.Message
}
