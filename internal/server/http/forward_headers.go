package httpserver

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// forwardHeadersFromFiber builds a header set that should be forwarded to downstream services.
// Keep it conservative to avoid surprises.
func forwardHeadersFromFiber(c *fiber.Ctx) http.Header {
	h := make(http.Header)
	for _, k := range []string{
		"Authorization",
		"X-Request-Id",
		"Accept",
		"Content-Type",
		"User-Agent",
		"X-Forwarded-For",
		"X-Real-IP",
	} {
		if v := c.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	return h
}
