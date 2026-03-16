package httpserver

import (
	"strings"
	"time"

	"anime/develop/backend/internal/observability"

	"github.com/gofiber/fiber/v2"
)

func (s *server) requestLogger(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	observability.Request(map[string]any{
		"method":      c.Method(),
		"path":        c.OriginalURL(),
		"status":      c.Response().StatusCode(),
		"latency_ms":  time.Since(start).Milliseconds(),
		"ip":          c.IP(),
		"user_agent":  c.Get("User-Agent"),
		"origin":      c.Get("Origin"),
		"clientToken": c.Get("X-Client-Token"),
	})
	return err
}

func (s *server) rateLimit(c *fiber.Ctx) error {
	key := c.IP() + ":" + time.Now().UTC().Format("200601021504")
	if s.publicRPM.Increment(key) > s.cfg.PublicRateLimitRPM {
		return c.Status(fiber.StatusTooManyRequests).JSON(errEnvelope("rate_limited", "Too many requests.", nil))
	}
	return c.Next()
}

func (s *server) requireClient(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Get("X-Client-Token"))
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(errEnvelope("client_token_required", "Anonymous session required.", nil))
	}
	client, err := s.store.FindClient(token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(errEnvelope("client_token_required", "Anonymous session required.", nil))
	}
	c.Locals("client_id", client.ID)
	c.Locals("client_token", client.Token)
	return c.Next()
}
