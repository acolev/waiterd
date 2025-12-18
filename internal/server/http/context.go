package httpserver

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var (
	reqStartUnix = time.Now().UnixNano()
	reqCounter   uint64
)

// makeReqID returns external X-Request-Id if provided, otherwise generates UUIDv4;
// if uuid generation fails, fallback to timestamp+counter.
func makeReqID(c *fiber.Ctx) string {
	if hdr := c.Get("X-Request-Id"); hdr != "" {
		return hdr
	}
	if v, err := uuid.NewRandom(); err == nil {
		return v.String()
	}
	n := atomic.AddUint64(&reqCounter, 1)
	return fmt.Sprintf("%x-%x", reqStartUnix, n)
}

// reqLogger returns printf-style logger prefixed with request id.
func reqLogger(c *fiber.Ctx) func(format string, args ...any) {
	reqID := makeReqID(c)
	return func(format string, args ...any) {
		log.Printf("[req=%s]"+format, append([]any{reqID}, args...)...)
	}
}
