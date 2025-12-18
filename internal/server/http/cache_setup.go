package httpserver

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"strings"
	"time"

	"waiterd/internal/config"
	appcache "waiterd/pkg/cache"
)

// SetupCache wires CacheInstance/DefaultCacheTTL based on gateway cache config.
// Returns cleanup function (no-op if cache not enabled).
func SetupCache(cfg config.Cache) (func(), error) {
	DefaultCacheTTL = parseTTL(cfg.TTL)

	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver != "redis" {
		// memory/disabled — leave CacheInstance nil
		return func() {}, nil
	}

	cacheCfg := appcache.Config{
		Addr:       fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:   cfg.Pass,
		DB:         cfg.Db,
		Prefix:     "waiterd",
		DefaultTTL: int(DefaultCacheTTL.Seconds()),
	}

	r, err := appcache.Init(context.Background(), cacheCfg)
	if err != nil {
		return nil, fmt.Errorf("init redis cache: %w", err)
	}

	CacheInstance = &redisCacheAdapter{rdb: r.Client}

	return func() { _ = r.Close() }, nil
}

type redisCacheAdapter struct {
	rdb *redis.Client
}

func (a *redisCacheAdapter) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := a.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

func (a *redisCacheAdapter) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return a.rdb.Set(ctx, key, data, ttl).Err()
}

func parseTTL(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	return 0
}
