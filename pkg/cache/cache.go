package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb        *redis.Client
	prefix     string
	defaultTTL time.Duration
}

func New(r *Redis) *Cache {
	ttl := time.Duration(r.Cfg.DefaultTTL) * time.Second
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Cache{
		rdb:        r.Client,
		prefix:     r.Cfg.Prefix,
		defaultTTL: ttl,
	}
}

func (c *Cache) Key(parts ...string) string {
	raw := ""
	for i, p := range parts {
		if i > 0 {
			raw += "|"
		}
		raw += p
	}
	sum := sha1.Sum([]byte(raw))
	return fmt.Sprintf("%s:%s", c.prefix, hex.EncodeToString(sum[:]))
}

func (c *Cache) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return true, json.Unmarshal(b, dst)
}

func (c *Cache) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

// Remember = cache-aside. Кэш не должен ломать логику: при ошибке кэша считаем miss.
func (c *Cache) Remember(ctx context.Context, key string, ttl time.Duration, dst any, fn func() (any, error)) error {
	if ok, err := c.GetJSON(ctx, key, dst); err == nil && ok {
		return nil
	}

	val, err := fn()
	if err != nil {
		return err
	}

	_ = c.SetJSON(ctx, key, val, ttl)

	b, _ := json.Marshal(val)
	return json.Unmarshal(b, dst)
}
