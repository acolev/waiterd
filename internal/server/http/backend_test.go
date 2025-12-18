package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

type stubCache struct {
	store    map[string][]byte
	getCount int32
	setCount int32
}

func (s *stubCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	atomic.AddInt32(&s.getCount, 1)
	if s.store == nil {
		return nil, false, nil
	}
	b, ok := s.store[key]
	return b, ok, nil
}

func (s *stubCache) Set(_ context.Context, key string, data []byte, _ time.Duration) error {
	atomic.AddInt32(&s.setCount, 1)
	if s.store == nil {
		s.store = make(map[string][]byte)
	}
	s.store[key] = data
	return nil
}

func TestProxyHTTP_CachesResponse(t *testing.T) {
	t.Cleanup(func() {
		CacheInstance = nil
		DefaultCacheTTL = 0
	})

	hits := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("pong"))
	}))
	t.Cleanup(srv.Close)

	services := map[string]config.Service{
		"svc": {Name: "svc", ProxyURL: srv.URL},
	}
	ep := config.Endpoint{Path: "/ping", Backend: &config.Backend{Service: "svc", Path: "/"}}

	cache := &stubCache{}
	CacheInstance = cache
	DefaultCacheTTL = time.Minute

	app := fiber.New()
	app.Get("/ping", makeEndpointHandler(services, ep))

	req := httptest.NewRequest("GET", "/ping", nil)
	if resp, err := app.Test(req); err != nil || resp.StatusCode != 200 {
		t.Fatalf("first call err=%v status=%v", err, resp.StatusCode)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("backend hits=%d want 1", hits)
	}
	if atomic.LoadInt32(&cache.setCount) != 1 {
		t.Fatalf("cache setCount=%d want 1", cache.setCount)
	}

	// second call should hit cache, no extra backend hits
	req2 := httptest.NewRequest("GET", "/ping", nil)
	if resp, err := app.Test(req2); err != nil || resp.StatusCode != 200 {
		t.Fatalf("second call err=%v status=%v", err, resp.StatusCode)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("backend hits=%d want still 1", hits)
	}
	if atomic.LoadInt32(&cache.getCount) < 1 {
		t.Fatalf("cache getCount not incremented")
	}
}
