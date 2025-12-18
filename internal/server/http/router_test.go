package httpserver

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

func TestRegisterRoutes_BackendPathParams(t *testing.T) {
	hits := int32(0)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("ok"))
	}))
	t.Cleanup(backend.Close)

	cfg := &config.FinalConfig{
		Services: []config.Service{{Name: "svc", ProxyURL: backend.URL}},
		Endpoints: []config.Endpoint{
			{Path: "/item/{id}", Method: http.MethodGet, Backend: &config.Backend{Service: "svc", Path: "/value"}},
		},
	}

	app := fiber.New()
	RegisterRoutes(app, cfg)

	req := httptest.NewRequest(http.MethodGet, "/item/123?foo=bar", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test err=%v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want %d", resp.StatusCode, http.StatusOK)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("backend hits=%d want 1", hits)
	}
}
