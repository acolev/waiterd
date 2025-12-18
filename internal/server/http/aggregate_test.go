package httpserver

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

// reuse simple stub cache

func TestAggregate_CachesFinalResponse(t *testing.T) {
	t.Cleanup(func() {
		CacheInstance = nil
		DefaultCacheTTL = 0
	})

	hitA, hitB := int32(0), int32(0)

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitA, 1)
		w.Write([]byte(`{"title":"hello"}`))
	}))
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitB, 1)
		w.Write([]byte(`{"body":"world"}`))
	}))
	t.Cleanup(func() {
		srvA.Close()
		srvB.Close()
	})

	services := map[string]config.Service{
		"a": {Name: "a", ProxyURL: srvA.URL},
		"b": {Name: "b", ProxyURL: srvB.URL},
	}

	ep := config.Endpoint{
		Path: "/mix",
		Calls: []config.AggCall{
			{Name: "post", Service: "a", Path: "/p"},
			{Name: "test", Service: "b", Path: "/t"},
		},
		ResponseMapping: map[string]string{
			"title":   "post.title",
			"message": "test.body",
		},
		CacheTTL: "1m",
	}

	CacheInstance = &stubCache{}
	DefaultCacheTTL = time.Minute

	app := fiber.New()
	app.Get("/mix", makeEndpointHandler(services, ep))

	req := httptest.NewRequest("GET", "/mix", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("first agg err=%v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if hitA != 1 || hitB != 1 {
		t.Fatalf("backend hits a=%d b=%d", hitA, hitB)
	}

	// second call should hit cache, no extra backend hits
	req2 := httptest.NewRequest("GET", "/mix", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("second agg err=%v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("status2=%d", resp2.StatusCode)
	}
	if hitA != 1 || hitB != 1 {
		t.Fatalf("backend hits after cache a=%d b=%d", hitA, hitB)
	}
}
