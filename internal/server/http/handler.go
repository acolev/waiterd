package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	"waiterd/internal/config"
)

// makeEndpointHandler решает, как обрабатывать endpoint: прямой backend или агрегация.
func makeEndpointHandler(services map[string]config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// в будущем можно подключить auth/middlewares через ep.Middlewares/ep.AuthRequired
		switch {
		case ep.Backend != nil:
			return httpBackendHandler(services, ep)(c)
		case len(ep.Calls) > 0:
			return makeAggregateHandler(services, ep)(c)
		default:
			return c.Status(http.StatusInternalServerError).SendString("endpoint is not configured (no backend/calls)")
		}
	}
}

// indexServices нормализует и индексирует сервисы по имени.
func indexServices(services []config.Service) map[string]config.Service {
	m := make(map[string]config.Service, len(services))
	for _, s := range services {
		if strings.TrimSpace(s.Transport) == "" {
			s.Transport = "http"
		}
		m[s.Name] = s
	}
	return m
}

// ======================
// PROXY (Backend)
// ======================

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

func proxyHTTP(c *fiber.Ctx, svc config.Service, ep config.Endpoint) error {
	logReq := reqLogger(c)
	// определяем TTL для этого endpoint
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

	// таймаут сервиса
	timeout := 5 * time.Second
	if svc.Timeout != "" {
		if d, err := time.ParseDuration(svc.Timeout); err == nil {
			timeout = d
		}
	}

	// base URL
	baseURL := svc.ProxyURL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	base, err := url.Parse(baseURL)
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

	// raw query — берём из OriginalURL
	rawQuery := ""
	if u, err := url.ParseRequestURI(c.OriginalURL()); err == nil {
		rawQuery = u.RawQuery
	}
	target.RawQuery = rawQuery

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

// ======================
// AGGREGATION (Calls)
// ======================

func makeAggregateHandler(services map[string]config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
				logReq("[waiterd][cache] hit key=%s path=%s", cacheKey, c.Path())
				var anyv any
				if err := json.Unmarshal(data, &anyv); err == nil {
					return c.JSON(anyv)
				}
				return c.SendString(string(data))
			}
		} else if ttlToUse > 0 {
			logReq("[waiterd][cache] disabled driver or instance nil; path=%s ttl=%s", c.Path(), ttlToUse)
		}

		perCall := make(map[string]any)

		var mu sync.Mutex
		g, gctx := errgroup.WithContext(c.Context())

		failOnError := true
		if ep.FailOnError != nil {
			failOnError = *ep.FailOnError
		}

		for _, call := range ep.Calls {
			call := call
			g.Go(func() error {
				startCall := time.Now()

				svc, ok := services[call.Service]
				if !ok {
					msg := fmt.Sprintf("unknown service %q", call.Service)
					logReq("[waiterd] aggregate call %s error: %s", call.Name, msg)
					if failOnError {
						return errors.New(msg)
					}
					mu.Lock()
					perCall[call.Name] = map[string]any{"_error": msg}
					mu.Unlock()
					return nil
				}

				transport := strings.ToLower(strings.TrimSpace(svc.Transport))
				if transport == "" {
					transport = "http"
				}
				if transport != "http" {
					msg := fmt.Sprintf("grpc in aggregate not implemented for service %q", svc.Name)
					logReq("[waiterd] aggregate call %s error: %s", call.Name, msg)
					if failOnError {
						return errors.New(msg)
					}
					mu.Lock()
					perCall[call.Name] = map[string]any{"_error": msg}
					mu.Unlock()
					return nil
				}

				resolvedPath := resolvePathTemplate(ep.Path, call.Path, c.Path())

				rawQuery := ""
				if u, err := url.ParseRequestURI(c.OriginalURL()); err == nil {
					rawQuery = u.RawQuery
				}

				methodToUse := call.Method
				if methodToUse == "" {
					methodToUse = http.MethodGet
				}

				targetURL := func() string {
					baseURL := svc.ProxyURL
					if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
						baseURL = "http://" + baseURL
					}
					base, err := url.Parse(baseURL)
					if err != nil {
						return baseURL
					}
					t := *base
					t.Path = singleJoinPath(base.Path, resolvedPath)
					t.RawQuery = rawQuery
					return t.String()
				}()

				bodyBytes, status, err := doHTTPCall(
					gctx,
					svc,
					call.Method,
					resolvedPath,
					rawQuery,
					nil,
				)
				if err != nil {
					logReq("[waiterd] aggregate call %s -> svc=%s error: %v", call.Name, svc.Name, err)
					if failOnError {
						return fmt.Errorf("aggregate call %s -> svc=%s error: %w", call.Name, svc.Name, err)
					}
					mu.Lock()
					perCall[call.Name] = map[string]any{"_error": err.Error()}
					mu.Unlock()
					return nil
				}

				if status >= 400 {
					if failOnError {
						logReq("[waiterd] aggregate call %s -> svc=%s returned status=%d -> aggregate will fail", call.Name, svc.Name, status)
						return fmt.Errorf("downstream status %d", status)
					}
					logReq("[waiterd] downstream call %s -> svc=%s returned status=%d (tolerated)", call.Name, svc.Name, status)
				}

				var value any
				if len(call.Mapping) == 0 {
					if err := json.Unmarshal(bodyBytes, &value); err != nil {
						value = string(bodyBytes)
					}
				} else {
					var decoded map[string]any
					if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
						value = string(bodyBytes)
					} else {
						mapped := make(map[string]any)
						for outKey, jsonField := range call.Mapping {
							if v, ok := decoded[jsonField]; ok {
								mapped[outKey] = v
							}
						}
						value = mapped
					}
				}

				if status >= 400 && value == nil {
					value = fmt.Sprintf("status=%d", status)
				}

				logReq("[waiterd][call] name=%s svc=%s url=%s method=%s status=%d in=%s", call.Name, svc.Name, targetURL, methodToUse, status, time.Since(startCall))

				mu.Lock()
				perCall[call.Name] = value
				if status >= 400 {
					perCall[call.Name+"_error"] = fmt.Sprintf("status=%d", status)
				}
				mu.Unlock()

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			logReq("[waiterd] aggregate error: %v", err)
			return c.Status(http.StatusBadGateway).SendString("backend error in aggregate")
		}

		var final any
		if len(ep.ResponseMapping) > 0 {
			out := make(map[string]any)
			for outKey, expr := range ep.ResponseMapping {
				parts := strings.SplitN(expr, ".", 2)
				callName := parts[0]
				callVal, ok := perCall[callName]
				if !ok {
					continue
				}
				if len(parts) == 1 {
					out[outKey] = callVal
					continue
				}
				fieldName := parts[1]
				if m, ok := callVal.(map[string]any); ok {
					if v, ok2 := m[fieldName]; ok2 {
						out[outKey] = v
					}
				}
			}
			final = out
		} else {
			final = perCall
		}

		if CacheInstance != nil && ttlToUse > 0 {
			if data, err := json.Marshal(final); err == nil {
				_ = CacheInstance.Set(c.UserContext(), cacheKey, data, ttlToUse)
			}
		}

		var errs []string
		for k, v := range perCall {
			if strings.HasSuffix(k, "_error") {
				errKey := strings.TrimSuffix(k, "_error")
				errStr := fmt.Sprintf("%s=%v", errKey, v)
				errs = append(errs, errStr)
			}
		}
		if len(errs) > 0 {
			logReq("[waiterd] aggregate completed with downstream errors: %s", strings.Join(errs, ", "))
		}

		return c.JSON(final)
	}
}

// ======================
// LOW-LEVEL HTTP
// ======================

func doHTTPCall(ctx context.Context, svc config.Service, method string, path string, rawQuery string, body io.Reader) ([]byte, int, error) {
	timeout := 5 * time.Second
	if svc.Timeout != "" {
		if d, err := time.ParseDuration(svc.Timeout); err == nil {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	baseURL := svc.ProxyURL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid proxy_url %q: %w", svc.ProxyURL, err)
	}

	target := *base
	target.Path = singleJoinPath(base.Path, path)
	target.RawQuery = rawQuery

	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}

	return data, resp.StatusCode, nil
}

// ======================
// HELPERS
// ======================

func grpcNotImplementedHandler(svc config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Printf("[waiterd] grpc transport for service=%s endpoint=%s %s not implemented yet", svc.Name, ep.Method, ep.Path)
		return c.Status(http.StatusNotImplemented).SendString("gRPC transport not implemented yet")
	}
}

func resolvePathTemplate(endpointPattern, callPattern, actualPath string) string {
	if !strings.Contains(callPattern, "{") {
		return callPattern
	}

	const placeholder = "{id}"

	if !strings.Contains(endpointPattern, placeholder) {
		return callPattern
	}

	epSegs := strings.Split(endpointPattern, "/")
	actSegs := strings.Split(actualPath, "/")
	callSegs := strings.Split(callPattern, "/")

	var epVals []string
	for i, seg := range epSegs {
		if strings.Contains(seg, placeholder) {
			if i < len(actSegs) {
				epVals = append(epVals, actSegs[i])
			} else {
				epVals = append(epVals, "")
			}
		}
	}

	if len(epVals) == 0 {
		return callPattern
	}

	callPlaceholders := 0
	for _, s := range callSegs {
		if strings.Contains(s, placeholder) {
			callPlaceholders++
		}
	}

	var reps []string
	if callPlaceholders <= len(epVals) {
		reps = epVals[len(epVals)-callPlaceholders:]
	} else {
		reps = make([]string, callPlaceholders)
		for i := 0; i < callPlaceholders; i++ {
			if i < len(epVals) {
				reps[i] = epVals[i]
			} else {
				reps[i] = epVals[len(epVals)-1]
			}
		}
	}

	ri := 0
	for i, s := range callSegs {
		if strings.Contains(s, placeholder) {
			callSegs[i] = strings.ReplaceAll(s, placeholder, reps[ri])
			ri++
		}
	}

	res := strings.Join(callSegs, "/")
	if res == "" {
		return "/"
	}
	return res
}

func singleJoinPath(a, b string) string {
	if a == "" && b == "" {
		return "/"
	}
	if a == "" {
		if !strings.HasPrefix(b, "/") {
			return "/" + b
		}
		return b
	}
	if b == "" {
		if !strings.HasPrefix(a, "/") {
			return "/" + a
		}
		return a
	}

	a = strings.TrimRight(a, "/")
	b = strings.TrimLeft(b, "/")
	return a + "/" + b
}

// CacheInstance — точка подключения кэш-адаптера и DefaultCacheTTL теперь в cache_stub.go

// copyHeaders копирует выбранные заголовки из Fiber запроса в http.Request.
func copyHeaders(c *fiber.Ctx, req *http.Request) {
	headerKeys := []string{"Content-Type", "Authorization", "Accept", "User-Agent", "X-Request-Id"}
	for _, hk := range headerKeys {
		if v := c.Get(hk); v != "" {
			req.Header.Set(hk, v)
		}
	}
}

// copyRespHeaders копирует заголовки из http.Response в Fiber ctx.
func copyRespHeaders(resp *http.Response, c *fiber.Ctx) {
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Set(k, v)
		}
	}
}

var (
	reqStartUnix = time.Now().UnixNano()
	reqCounter   uint64
)

// makeReqID returns external X-Request-Id if provided, otherwise generates unique per-process hex id (start-timestamp + counter).
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
