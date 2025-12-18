package httpserver

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"

	"waiterd/internal/config"
)

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

func grpcNotImplementedHandler(svc config.Service, ep config.Endpoint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Printf("[waiterd] grpc transport for service=%s endpoint=%s %s not implemented yet", svc.Name, ep.Method, ep.Path)
		return c.Status(http.StatusNotImplemented).SendString("gRPC transport not implemented yet")
	}
}

// decodeWithMapping декодирует тело JSON и применяет mapping; если mapping пуст — возвращает json или string.
func decodeWithMapping(body []byte, mapping map[string]string) any {
	if len(mapping) == 0 {
		var anyJSON any
		if err := json.Unmarshal(body, &anyJSON); err == nil {
			return anyJSON
		}
		return string(body)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return string(body)
	}

	mapped := make(map[string]any)
	for outKey, jsonField := range mapping {
		if v, ok := decoded[jsonField]; ok {
			mapped[outKey] = v
		}
	}
	return mapped
}

// buildAggregateResponse строит финальный объект по response_mapping (или возвращает perCall).
func buildAggregateResponse(mapping map[string]string, perCall map[string]any) any {
	if len(mapping) == 0 {
		return perCall
	}

	out := make(map[string]any)
	for outKey, expr := range mapping {
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
	return out
}
