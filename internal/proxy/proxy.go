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

// NewHTTPClient создаёт http.Client с SOCKS5-прокси.
// Если cfg.Proxy.URL пуст, возвращает стандартный клиент.
func NewHTTPClient(cfg *config.ProxyConfig) (*http.Client, error) {
	// Базовый диалектор: стандартный или IPv6-first
	var base proxy.Dialer = proxy.Direct
	if cfg.PreferIPv6 {
		base = &ipv6FirstDialer{next: proxy.Direct}
	}

	if !cfg.Enabled() {
		transport := &http.Transport{
			DialContext:           base.(proxy.ContextDialer).DialContext,
			ResponseHeaderTimeout: 30 * time.Second,
		}
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}, nil
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

	// Базовый диалектор для SOCKS5
	var baseDialer proxy.Dialer = proxy.Direct
	if cfg.PreferIPv6 {
		baseDialer = &ipv6FirstDialer{next: proxy.Direct}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, baseDialer)
	if err != nil {
		return nil, fmt.Errorf("proxy socks5 dialer: %w", err)
	}

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

// ipv6FirstDialer — обёртка над proxy.Dialer, которая при коннекте к хосту
// резолвит адреса и пробует IPv6 (AAAA) раньше IPv4 (A).
type ipv6FirstDialer struct {
	next proxy.Dialer
}

func (d *ipv6FirstDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *ipv6FirstDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// fallback
		conn, dialErr := d.next.(proxy.ContextDialer).DialContext(ctx, network, addr)
		return conn, dialErr
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		conn, dialErr := d.next.(proxy.ContextDialer).DialContext(ctx, network, addr)
		return conn, dialErr
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

	var lastErr error
	for _, ip := range sorted {
		target := net.JoinHostPort(ip.String(), port)
		conn, dialErr := d.next.(proxy.ContextDialer).DialContext(ctx, network, target)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

// Убеждаемся, что ipv6FirstDialer реализует proxy.ContextDialer
var _ proxy.ContextDialer = (*ipv6FirstDialer)(nil)
