package httpserver

import (
	"context"
	"time"
)

// Cache interface to decouple from concrete cache implementations.
type cacheInterface interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
}

// CacheInstance can be wired from main once cache is ready.
var CacheInstance cacheInterface

// DefaultCacheTTL is used for aggregate endpoints when not specified.
var DefaultCacheTTL = 0 * time.Second
