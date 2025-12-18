package httpserver

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"waiterd/internal/config"
)

// Server wraps Fiber app and configuration.
type Server struct {
	app *fiber.App
	cfg *config.FinalConfig
}

// New builds a Fiber server with common middlewares.
func New(cfg *config.FinalConfig) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "waiterd",
		ReadTimeout:  time.Duration(cfg.Gateway.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Gateway.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Gateway.IdleTimeoutSec) * time.Second,
	})

	app.Use(recover.New())
	//app.Use(logger.New())

	RegisterRoutes(app, cfg)

	return &Server{app: app, cfg: cfg}
}

// Start runs Fiber server and handles graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	addr := cfgAddress(s.cfg.Gateway.Address)
	log.Printf("[waiterd] listening on %s", addr)

	// start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Listen(addr)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.Gateway.ShutdownTimeoutSec)*time.Second)
		defer cancel()
		return s.app.ShutdownWithContext(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func cfgAddress(addr string) string {
	if addr == "" {
		return ":" // default Fiber listens on 0.0.0.0
	}
	return addr
}
