package httpserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

// proxyHTTP проксирует запрос к backend-сервису с учётом cache_ttl и логирует с reqID.
func proxyHTTP(c *fiber.Ctx, svc config.Service, ep config.Endpoint) error {
	logReq := reqLogger(c)

	ttlToUse := DefaultCacheTTL
	if ep.CacheTTL != "" {
		if d, err := time.ParseDuration(ep.CacheTTL); err == nil {
			ttlToUse = d
		} else if secs, err := strconv.Atoi(ep.CacheTTL); err == nil {
			ttlToUse = time.Duration(secs) * time.Second
		}
	}

	cacheKey := c.Method() + ":" + c.OriginalURL()
	if CacheInstance != nil && ttlToUse > 0 {
		if data, ok, err := CacheInstance.Get(c.UserContext(), cacheKey); err == nil && ok {
			logReq("[waiterd][cache] hit key=%s path=%s svc=%s", cacheKey, c.Path(), svc.Name)
			return c.SendStream(bytes.NewReader(data))
		}
	}

	timeout := serviceTimeout(svc)
	base, err := parseBaseURL(svc.ProxyURL)
	if err != nil {
		logReq("[waiterd] invalid proxy_url %q for service %q: %v", svc.ProxyURL, svc.Name, err)
		return c.Status(http.StatusInternalServerError).SendString("invalid backend url")
	}

	method := ep.Backend.Method
	if method == "" {
		method = c.Method()
	}

	target := *base
	target.Path = singleJoinPath(base.Path, ep.Backend.Path)
	target.RawQuery = rawQueryFromOriginal(c.OriginalURL())

	logReq("[waiterd][call] svc=%s target=%s method=%s", svc.Name, target.String(), method)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	body := bytes.NewReader(c.Body())
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		logReq("[waiterd] new backend request error: %v", err)
		return c.Status(http.StatusInternalServerError).SendString("backend request build error")
	}

	copyHeaders(c, req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logReq("[waiterd] backend %s %s -> %s error: %v", method, target.String(), svc.Name, err)
		return c.Status(http.StatusBadGateway).SendString("backend unavailable")
	}
	defer resp.Body.Close()

	copyRespHeaders(resp, c)

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		logReq("[waiterd] read response error: %v", readErr)
		return c.Status(http.StatusBadGateway).SendString("backend read error")
	}

	c.Status(resp.StatusCode)
	if _, err := c.Write(bodyBytes); err != nil {
		logReq("[waiterd] write response error: %v", err)
	}

	if CacheInstance != nil && ttlToUse > 0 && resp.StatusCode < 500 {
		_ = CacheInstance.Set(c.UserContext(), cacheKey, bodyBytes, ttlToUse)
	}

	return nil
}

func serviceTimeout(svc config.Service) time.Duration {
	timeout := 5 * time.Second
	if svc.Timeout != "" {
		if d, err := time.ParseDuration(svc.Timeout); err == nil {
			timeout = d
		}
	}
	return timeout
}

func parseBaseURL(raw string) (*url.URL, error) {
	baseURL := raw
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return url.Parse(baseURL)
}

func rawQueryFromOriginal(original string) string {
	if u, err := url.ParseRequestURI(original); err == nil {
		return u.RawQuery
	}
	return ""
}

// httpBackendHandler подбирает транспорт (пока только http) и делегирует в proxyHTTP.
func httpBackendHandler(services map[string]config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		b := ep.Backend
		if b == nil {
			return c.Status(http.StatusInternalServerError).SendString("backend not configured")
		}

		svc, ok := services[b.Service]
		if !ok {
			return c.Status(http.StatusBadGateway).SendString("unknown backend service")
		}

		transport := strings.ToLower(strings.TrimSpace(svc.Transport))
		if transport == "" {
			transport = "http"
		}

		switch transport {
		case "grpc":
			return grpcNotImplementedHandler(svc, ep)(c)
		default:
			return proxyHTTP(c, svc, ep)
		}
	}
}
