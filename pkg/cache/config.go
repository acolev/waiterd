package cache

import "waiterd/pkg/cfg"

type Config struct {
	Addr       string
	Password   string
	DB         int
	Prefix     string
	DefaultTTL int // seconds
}

func LoadConfigFromEnv() Config {
	return Config{
		Addr:       cfg.String("REDIS_ADDR", "127.0.0.1:6379"),
		Password:   cfg.String("REDIS_PASSWORD", ""),
		DB:         cfg.Int("REDIS_DB", 0),
		Prefix:     cfg.String("REDIS_PREFIX", "ninjacore"),
		DefaultTTL: cfg.Int("REDIS_DEFAULT_TTL_SEC", 60),
	}
}
