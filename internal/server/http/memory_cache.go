package httpserver

import (
	"context"
	"sync"
	"time"
)

// memoryCacheAdapter is a small in-memory TTL cache implementing cacheInterface.
// It's used when cache.driver=memory.
// NOTE: It's intentionally simple (single process, best-effort cleanup).
// It is safe for concurrent use.

type memoryCacheAdapter struct {
	mu    sync.RWMutex
	items map[string]memItem
}

type memItem struct {
	b   []byte
	exp time.Time
}

func newMemoryCacheAdapter() *memoryCacheAdapter {
	return &memoryCacheAdapter{items: make(map[string]memItem)}
}

func (m *memoryCacheAdapter) Get(ctx context.Context, key string) ([]byte, bool, error) {
	_ = ctx
	m.mu.RLock()
	it, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !it.exp.IsZero() && time.Now().After(it.exp) {
		m.mu.Lock()
		// re-check
		it2, ok2 := m.items[key]
		if ok2 && !it2.exp.IsZero() && time.Now().After(it2.exp) {
			delete(m.items, key)
		}
		m.mu.Unlock()
		return nil, false, nil
	}
	// return a copy to avoid external mutation
	out := make([]byte, len(it.b))
	copy(out, it.b)
	return out, true, nil
}

func (m *memoryCacheAdapter) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	_ = ctx
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	b := make([]byte, len(data))
	copy(b, data)
	m.mu.Lock()
	m.items[key] = memItem{b: b, exp: exp}
	m.mu.Unlock()
	return nil
}

func (m *memoryCacheAdapter) Clear() {
	m.mu.Lock()
	clear(m.items)
	m.mu.Unlock()
}
