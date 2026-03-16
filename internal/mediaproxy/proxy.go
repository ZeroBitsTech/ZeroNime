package mediaproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var allowedHosts = []string{
	"pixeldrain.com",
	"kitasan.my.id",
	"amiya.my.id",
	"anisphia.my.id",
	"r2.cloudflarestorage.com",
}

type Proxy struct {
	client *http.Client
}

func New() *Proxy {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Proxy{client: &http.Client{Transport: transport}}
}

func (p *Proxy) Allowed(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range allowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	if strings.HasSuffix(host, ".my.id") {
		return true
	}
	return false
}

func (p *Proxy) Fetch(ctx context.Context, rawURL, rangeHeader string) (*http.Response, error) {
	if !p.Allowed(rawURL) {
		return nil, fmt.Errorf("host not allowed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", refererFor(rawURL))
	if strings.TrimSpace(rangeHeader) != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("media status %d", resp.StatusCode)
	}
	return resp, nil
}

func refererFor(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "https://pixeldrain.com/"
	}

	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.HasSuffix(host, ".my.id"), strings.HasSuffix(host, ".r2.cloudflarestorage.com"):
		return "https://v17.kuramanime.ink/"
	default:
		return "https://pixeldrain.com/"
	}
}
