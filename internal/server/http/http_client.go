package httpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"waiterd/internal/config"
)

// doHTTPCall делает HTTP вызов к сервису с respect timeout и возвращает тело+статус.
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
