package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	"waiterd/internal/config"
)

// makeAggregateHandler обрабатывает агрегацию calls, кэширует итог и логирует с reqID.
func makeAggregateHandler(services map[string]config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logReq := reqLogger(c)

		ttlToUse := DefaultCacheTTL
		if ep.CacheTTL != "" {
			if d, err := time.ParseDuration(ep.CacheTTL); err == nil {
				ttlToUse = d
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

				if strings.TrimSpace(strings.ToLower(svc.Transport)) == "grpc" {
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
				rawQuery := rawQueryFromOriginal(c.OriginalURL())
				methodToUse := call.Method
				if methodToUse == "" {
					methodToUse = http.MethodGet
				}

				targetURL := buildTargetURL(svc.ProxyURL, resolvedPath, rawQuery)

				fwd := forwardHeadersFromFiber(c)
				bodyBytes, status, err := doHTTPCall(
					gctx,
					svc,
					call.Method,
					resolvedPath,
					rawQuery,
					nil,
					fwd,
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

				value := decodeWithMapping(bodyBytes, call.Mapping)
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

		final := buildAggregateResponse(ep.ResponseMapping, perCall)

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

func buildTargetURL(proxy string, path string, rawQuery string) string {
	baseURL := proxy
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	t := *base
	t.Path = singleJoinPath(base.Path, path)
	t.RawQuery = rawQuery
	return t.String()
}
