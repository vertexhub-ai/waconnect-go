<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white" alt="WhatsApp">
  <img src="https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker">
</p>

<h1 align="center">🚀 WAConnect Go</h1>

<p align="center">
  <strong>by <a href="https://github.com/vertexhub">VertexHub</a></strong>
</p>

<p align="center">
  <em>WhatsApp API Gateway - Fast, Simple, n8n-Ready</em>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#api-endpoints">API</a> •
  <a href="#dashboard">Dashboard</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License: MIT">
  <img src="https://img.shields.io/badge/Made%20by-VertexHub-blue" alt="Made by VertexHub">
  <img src="https://img.shields.io/badge/Version-1.0.0-orange" alt="Version">
</p>

---

## Features

- 🚀 **Fast** - Single binary, ~14MB, starts in < 100ms
- 📱 **Multi-session** - Manage multiple WhatsApp connections
- 🔌 **n8n-Ready** - Webhooks with HMAC signatures
- 🎨 **Dashboard** - Web UI for session management
- 🐳 **Docker** - One-command deployment
- 🔒 **Secure** - API key authentication, HTTPS ready

## Quick Start

### Binary

```bash
# Build
go build -o waconnect ./cmd/server

# Run
PORT=3200 API_KEY=your-secret-key ./waconnect
```

### Docker

```bash
# Build and run
docker compose up -d

# Or build only
docker build -t waconnect-go .
docker run -p 3200:3200 -v ./sessions:/app/sessions waconnect-go
```

### Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/vertexhub/waconnect-go.git
cd waconnect-go

# Start
docker compose up -d

# View logs
docker compose logs -f
```

## API Endpoints

### Health
```
GET /health
```

### Sessions
```
POST   /api/v1/session/create    # Create new session
GET    /api/v1/session           # List all sessions
GET    /api/v1/session/:id       # Get session info
GET    /api/v1/session/:id/qr    # Get QR code
GET    /api/v1/session/:id/status # Get status
DELETE /api/v1/session/:id       # Delete session
```

### Messages
```
POST /api/v1/send/text       # Send text message
POST /api/v1/send/media      # Send media (image, video, document)
POST /api/v1/send/location   # Send location
```

### Webhooks
```
GET    /api/v1/webhooks          # List webhooks
POST   /api/v1/webhooks          # Create webhook
DELETE /api/v1/webhooks/:id      # Delete webhook
POST   /api/v1/webhooks/:id/test # Test webhook
GET    /api/v1/webhooks/events   # Available events
```

## Authentication

All API endpoints require `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:3200/api/v1/session
```

## Webhooks (n8n Integration)

### Register Webhook
```bash
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://n8n.example.com/webhook/xxx", "events": ["message.received"], "secret": "my-secret"}' \
  http://localhost:3200/api/v1/webhooks
```

### Event Types
| Event | Description |
|-------|-------------|
| `session.connected` | Session authenticated |
| `session.disconnected` | Session disconnected |
| `session.qr_ready` | QR code available |
| `message.received` | Message received |
| `message.sent` | Message sent |
| `message.delivered` | Message delivered |
| `message.read` | Message read |
| `*` | All events |

### Webhook Payload
```json
{
  "event": "message.received",
  "timestamp": "2026-01-10T10:00:00Z",
  "webhookId": "wh_abc123",
  "signature": "sha256=...",
  "data": {
    "from": "5511999999999@c.us",
    "text": "Hello!",
    "sessionId": "my-session"
  }
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3200` | HTTP server port |
| `API_KEY` | `dev-api-key` | API authentication key |
| `SESSION_DIR` | `./sessions` | Session data directory |
| `DASHBOARD_USER` | `admin` | Dashboard username |
| `DASHBOARD_PASS` | `waconnect123` | Dashboard password |

## Dashboard

Access the web dashboard at: `http://localhost:3200/dashboard`

Features:
- 📊 Real-time session stats
- 📱 QR code display
- 🔄 Auto-refresh every 5 seconds
- 🎨 Modern dark theme

## Project Structure

```
waconnect-go/
├── cmd/server/          # Entry point
├── internal/
│   ├── core/            # Noise Protocol, Protobuf, WebSocket
│   ├── api/             # REST handlers (Fiber)
│   ├── client/          # Session management
│   └── webhook/         # Event dispatcher
├── public/              # Dashboard HTML/CSS/JS
├── Dockerfile           # Multi-stage build
├── docker-compose.yml   # Production config
└── LICENSE              # MIT License
```

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Credits

Developed with ❤️ by **[VertexHub](https://github.com/vertexhub)**

## License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <sub>Made with ❤️ by <a href="https://github.com/vertexhub">VertexHub</a></sub>
</p>
