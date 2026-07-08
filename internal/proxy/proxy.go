// Package proxy — SOCKS5-прокси для Telegram API.
package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"github.com/akrhin/keenetic-wg-bot/internal/config"
)

// NewHTTPClient создаёт http.Client с SOCKS5-прокси.
// Если cfg.Proxy.URL пуст, возвращает стандартный клиент.
func NewHTTPClient(cfg *config.ProxyConfig) (*http.Client, error) {
	if !cfg.Enabled() {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	proxyURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("proxy url parse: %w", err)
	}

	var auth *proxy.Auth
	if cfg.Username != "" {
		auth = &proxy.Auth{
			User:     cfg.Username,
			Password: cfg.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("proxy socks5 dialer: %w", err)
	}

	// Типовое приведение: SOCKS5-диалектор реализует proxy.ContextDialer.
	ctxDialer := dialer.(proxy.ContextDialer)

	transport := &http.Transport{
		DialContext:           ctxDialer.DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}
