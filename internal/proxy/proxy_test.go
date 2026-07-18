// Package proxy_test — unit tests for SOCKS5 proxy and IPv6-first dialer.
package proxy_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/akrhin/keenetic-wg-bot/internal/config"
	"github.com/akrhin/keenetic-wg-bot/internal/proxy"
)

func TestNewHTTPClient_NoProxyNoIPv6(t *testing.T) {
	cfg := &config.Config{}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != 90*time.Second {
		t.Errorf("timeout = %v, want 90s", client.Timeout)
	}
}

func TestNewHTTPClient_IPv6First(t *testing.T) {
	cfg := &config.Config{
		Network: config.NetworkConfig{PreferIPv6: true},
	}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != 90*time.Second {
		t.Errorf("timeout = %v, want 90s", client.Timeout)
	}
	if client.Transport == nil {
		t.Fatal("transport is nil")
	}
}

func TestNewHTTPClient_ProxyNoAuth(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			Enabled: true,
			URL:     "socks5://127.0.0.1:1080",
		},
	}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != 90*time.Second {
		t.Errorf("timeout = %v, want 90s", client.Timeout)
	}
}

func TestNewHTTPClient_InvalidProxyURL(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			Enabled: true,
			URL:     "://invalid",
		},
	}
	_, err := proxy.NewHTTPClient(cfg)
	if err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}

func TestNewHTTPClient_ProxyWithAuth(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			Enabled:  true,
			URL:      "socks5://127.0.0.1:1080",
			Username: "user",
			Password: "pass",
		},
	}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

func TestNewHTTPClient_ProxyWithIPv6(t *testing.T) {
	cfg := &config.Config{
		Network: config.NetworkConfig{PreferIPv6: true},
		Proxy: config.ProxyConfig{
			Enabled: true,
			URL:     "socks5://127.0.0.1:1080",
		},
	}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

func TestNewHTTPClient_ProxyDialerType(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			Enabled: true,
			URL:     "socks5://127.0.0.1:1080",
		},
	}
	client, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("transport is not *http.Transport")
	}
	if tr.DialContext == nil {
		t.Fatal("DialContext is nil")
	}
}
