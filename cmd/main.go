package main

import (
	"context"
	"github.com/joho/godotenv"
	"log"
	"os/signal"
	"syscall"
	"time"
	"waiterd/internal/config"
	httpserver "waiterd/internal/server/http"
	"waiterd/pkg/cfg"
	"waiterd/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	env := cfg.String("APP_ENV", "dev")

	cleanup := logger.Setup(env)
	defer cleanup()

	configPath := cfg.String("APP_CONFIG", "config.yaml")

	conf, err := config.Build(configPath)
	if err != nil {
		log.Fatalf("failed to build config: %v", err)
	}

	cacheCleanup, err := httpserver.SetupCache(conf.Cache)
	if err != nil {
		log.Fatalf("failed to init cache: %v", err)
	}
	defer cacheCleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := httpserver.New(conf)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// wait for signal
	<-ctx.Done()
	// give some time for graceful shutdown
	time.Sleep(time.Second)
}
