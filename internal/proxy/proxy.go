// Package proxy — SOCKS5-прокси для Telegram API.
package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"github.com/akrhin/keenetic-wg-bot/internal/config"
)

// NewHTTPClient creates http.Client with optional SOCKS5 proxy and/or IPv6-first dial.
func NewHTTPClient(cfg *config.Config) (*http.Client, error) {
	// Без прокси — обычный клиент (с опциональным IPv6-first)
	if !cfg.Proxy.IsEnabled() {
		if cfg.Network.PreferIPv6 {
			return &http.Client{
				Transport: &http.Transport{
					DialContext:           (&ipv6FirstDialer{}).DialContext,
					ResponseHeaderTimeout: 30 * time.Second,
				},
				Timeout: 30 * time.Second,
			}, nil
		}
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	// Прокси настроен и включён
	proxyURL, err := url.Parse(cfg.Proxy.URL)
	if err != nil {
		return nil, fmt.Errorf("proxy url parse: %w", err)
	}

	var auth *proxy.Auth
	if cfg.Proxy.Username != "" {
		auth = &proxy.Auth{
			User:     cfg.Proxy.Username,
			Password: cfg.Proxy.Password,
		}
	}

	// Базовый диалектор для SOCKS5
	var base proxy.Dialer = proxy.Direct
	if cfg.Network.PreferIPv6 {
		base = &ipv6FirstDialer{next: proxy.Direct}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, base)
	if err != nil {
		return nil, fmt.Errorf("proxy socks5 dialer: %w", err)
	}

	ctxDialer := dialer.(proxy.ContextDialer)

	return &http.Client{
		Transport: &http.Transport{
			DialContext:           ctxDialer.DialContext,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		Timeout: 60 * time.Second,
	}, nil
}

// ipv6FirstDialer — обёртка, резолвит AAAA (IPv6) раньше A (IPv4).
type ipv6FirstDialer struct {
	next proxy.Dialer
}

func (d *ipv6FirstDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *ipv6FirstDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return d.resolveAndDial(ctx, network, addr)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return d.resolveAndDial(ctx, network, addr)
	}

	// IPv6 первыми
	var sorted []net.IP
	for _, ip := range ips {
		if ip.IP.To4() == nil {
			sorted = append(sorted, ip.IP)
		}
	}
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			sorted = append(sorted, ip.IP)
		}
	}

	nextCtx := d.next
	if nextCtx == nil {
		nextCtx = proxy.Direct
	}

	var lastErr error
	for _, ip := range sorted {
		target := net.JoinHostPort(ip.String(), port)
		conn, dialErr := nextCtx.(proxy.ContextDialer).DialContext(ctx, network, target)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

func (d *ipv6FirstDialer) resolveAndDial(ctx context.Context, network, addr string) (net.Conn, error) {
	nextCtx := d.next
	if nextCtx == nil {
		nextCtx = proxy.Direct
	}
	return nextCtx.(proxy.ContextDialer).DialContext(ctx, network, addr)
}

var _ proxy.ContextDialer = (*ipv6FirstDialer)(nil)
