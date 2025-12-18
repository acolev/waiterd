package httpserver

import (
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

var pathParamRegex = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// RegisterRoutes строит маршруты на основе конфигурации.
func RegisterRoutes(app *fiber.App, cfg *config.FinalConfig) {
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	if strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) == "dev" {
		app.Get("/debug/config", func(c *fiber.Ctx) error { return c.JSON(cfg) })
	}

	services := indexServices(cfg.Services)

	for _, ep := range cfg.Endpoints {
		ep := ep // захватываем для замыкания

		if ep.Backend == nil && len(ep.Calls) == 0 {
			log.Printf("[waiterd] endpoint %s %s не имеет backend/calls — пропускаю", ep.Method, ep.Path)
			continue
		}

		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" {
			method = http.MethodGet
		}

		path := fiberPath(ep.Path)
		h := makeEndpointHandler(services, ep)

		log.Printf("[waiterd] register endpoint %s %s", method, path)
		switch method {
		case http.MethodGet:
			app.Get(path, h)
		case http.MethodPost:
			app.Post(path, h)
		case http.MethodPut:
			app.Put(path, h)
		case http.MethodPatch:
			app.Patch(path, h)
		case http.MethodDelete:
			app.Delete(path, h)
		default:
			log.Printf("[waiterd] unsupported method %q for path %q, skipping", method, path)
		}
	}
}

func fiberPath(path string) string {
	if path == "" {
		return path
	}
	return pathParamRegex.ReplaceAllString(path, ":$1")
}
