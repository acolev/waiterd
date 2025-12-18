package httpserver

import (
	"net/http"
)

// cachedHTTPResponse is what we store in cache for proxied responses.
// It includes status + selected headers + body.
// We intentionally do NOT store Set-Cookie for safety.
// (If you need it later, we can add an allow-list.)
type cachedHTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body"`
}

var cachedHeaderAllowList = map[string]struct{}{
	"Content-Type":     {},
	"Cache-Control":    {},
	"ETag":             {},
	"Last-Modified":    {},
	"Vary":             {},
	"Content-Language": {},
	// Note: do not cache Set-Cookie by default.
}

func extractCacheableHeaders(h http.Header) map[string]string {
	out := make(map[string]string)
	for k, vals := range h {
		if _, ok := cachedHeaderAllowList[k]; !ok {
			continue
		}
		if len(vals) == 0 {
			continue
		}
		out[k] = vals[0]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
