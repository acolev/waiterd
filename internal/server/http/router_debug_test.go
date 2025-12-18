package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

func TestRegisterRoutes_DebugConfig_DevOnly(t *testing.T) {
	cfg := &config.FinalConfig{}

	// non-dev: should be 404
	t.Setenv("APP_ENV", "prod")
	app := fiber.New()
	RegisterRoutes(app, cfg)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/debug/config", nil))
	if err != nil {
		t.Fatalf("req err: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want %d", resp.StatusCode, http.StatusNotFound)
	}

	// dev: should be 200
	t.Setenv("APP_ENV", "dev")
	app2 := fiber.New()
	RegisterRoutes(app2, cfg)
	resp2, err := app2.Test(httptest.NewRequest(http.MethodGet, "/debug/config", nil))
	if err != nil {
		t.Fatalf("req2 err: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status2=%d want %d", resp2.StatusCode, http.StatusOK)
	}
}
