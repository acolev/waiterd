package httpserver

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheAdapter_TTL(t *testing.T) {
	m := newMemoryCacheAdapter()
	ctx := context.Background()

	if err := m.Set(ctx, "k", []byte("v"), 20*time.Millisecond); err != nil {
		t.Fatalf("set: %v", err)
	}

	b, ok, err := m.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || string(b) != "v" {
		t.Fatalf("ok=%v b=%q", ok, string(b))
	}

	time.Sleep(30 * time.Millisecond)
	_, ok, err = m.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if ok {
		t.Fatalf("expected expired key")
	}
}
