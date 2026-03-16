package imageproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var allowedHosts = []string{
	"otakudesu.blog",
	"otakudesu.best",
	"otakudesu.cloud",
	"kuramanime.ink",
	"kuramanime.work",
	"kuramanime.tel",
	"i0.wp.com",
	"nyomo.my.id",
	"pixeldrain.com",
	"kitasan.my.id",
	"myanimelist.net",
}

type Proxy struct {
	client *http.Client
}

func New(timeout time.Duration) *Proxy {
	return &Proxy{client: &http.Client{Timeout: timeout}}
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
	return false
}

func (p *Proxy) Fetch(rawURL string) ([]byte, string, error) {
	if !p.Allowed(rawURL) {
		return nil, "", fmt.Errorf("host not allowed")
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Referer", refererFor(rawURL))
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("image status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return body, contentType, nil
}

func refererFor(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "https://otakudesu.best/"
	}

	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.Contains(host, "kuramanime"):
		return "https://v17.kuramanime.ink/"
	case strings.Contains(host, "myanimelist"):
		return "https://myanimelist.net/"
	default:
		return "https://otakudesu.best/"
	}
}
