package middleware

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// APIKeyAuth middleware validates API key
func APIKeyAuth() fiber.Handler {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = "dev-api-key" // Default for development
	}

	return func(c *fiber.Ctx) error {
		// Skip auth for certain paths
		path := c.Path()
		if strings.HasPrefix(path, "/dashboard") || 
		   strings.HasPrefix(path, "/health") ||
		   strings.HasPrefix(path, "/docs") {
			return c.Next()
		}

		// Get API key from header
		key := c.Get("X-API-Key")
		if key == "" {
			// Try Authorization header
			auth := c.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		// Validate key
		if key != apiKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid or missing API key",
			})
		}

		return c.Next()
	}
}

// DashboardAuth middleware for dashboard authentication
func DashboardAuth() fiber.Handler {
	username := os.Getenv("DASHBOARD_USER")
	password := os.Getenv("DASHBOARD_PASS")

	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "waconnect123"
	}

	return func(c *fiber.Ctx) error {
		// Check session cookie
		session := c.Cookies("session")
		if session != "" && session == generateSessionToken(username, password) {
			return c.Next()
		}

		// Try basic auth from Authorization header
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Basic ") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 && parts[0] == username && parts[1] == password {
					// Set session cookie
					c.Cookie(&fiber.Cookie{
						Name:     "session",
						Value:    generateSessionToken(username, password),
						MaxAge:   86400 * 7, // 7 days
						Secure:   false,
						HTTPOnly: true,
					})
					return c.Next()
				}
			}
		}

		// Request authentication
		c.Set("WWW-Authenticate", `Basic realm="WAConnect Dashboard"`)
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}
}

func generateSessionToken(username, password string) string {
	// Simple token generation - in production use proper JWT
	return "session_" + username
}
