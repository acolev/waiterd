package httpserver

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

// makeEndpointHandler решает, как обрабатывать endpoint: прямой backend или агрегация.
func makeEndpointHandler(services map[string]config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		switch {
		case ep.Backend != nil:
			return httpBackendHandler(services, ep)(c)
		case len(ep.Calls) > 0:
			return makeAggregateHandler(services, ep)(c)
		default:
			return c.Status(http.StatusInternalServerError).SendString("endpoint is not configured (no backend/calls)")
		}
	}
}

// indexServices нормализует и индексирует сервисы по имени.
func indexServices(services []config.Service) map[string]config.Service {
	m := make(map[string]config.Service, len(services))
	for _, s := range services {
		if strings.TrimSpace(s.Transport) == "" {
			s.Transport = "http"
		}
		m[s.Name] = s
	}
	return m
}
