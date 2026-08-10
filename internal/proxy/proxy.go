// Package proxy — SOCKS5-прокси для Telegram API.
//
// HTTP-клиент настраивается с жёсткими таймаутами:
//   - резолв DNS — не более 5с на запрос (не виснет на мёртвом 127.0.0.1:53);
//   - установка TCP-соединения — не более 15с;
//   - idle keep-alive соединения выживают максимум 90с и не копятся.
//
// Это критично для long-polling: при глюке сети соединения «протухают»,
// и без этих таймаутов запрос getUpdates висел бы бесконечно.
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

const (
	// dnsTimeout — таймаут одного DNS-запроса (по умолчанию Go может висеть дольше).
	dnsTimeout = 5 * time.Second
	// dialTimeout — таймаут установки TCP-соединения.
	dialTimeout = 15 * time.Second
	// idleConnTimeout — сколько живёт бездействующее keep-alive соединение.
	idleConnTimeout = 90 * time.Second
	// responseHeaderTimeout — верхняя граница ожидания заголовков ответа.
	// long-polling держит запрос открытым до 60с, 75с даёт запас.
	responseHeaderTimeout = 75 * time.Second
	// clientTimeout — общий таймаут HTTP-запроса.
	clientTimeout = 90 * time.Second
)

// newResolver возвращает резолвер с таймаутом на каждый DNS-запрос.
// Прямой системный резолвер (net.DefaultResolver) не имеет жёсткого таймаута:
// при зависшем локальном DNS (dnsmasq на роутере) он блокировал весь dial.
func newResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: dnsTimeout}
			return d.DialContext(ctx, network, address)
		},
	}
}

// newDialer возвращает net.Dialer с таймаутами и кастомным резолвером.
func newDialer(resolver *net.Resolver) *net.Dialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
		Resolver:  resolver,
	}
}

// newTransport собирает http.Transport с управляемым пулом соединений.
func newTransport(dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) *http.Transport {
	return &http.Transport{
		DialContext:           dialContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleConnTimeout,
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   2,
		ForceAttemptHTTP2:     false,
	}
}

// NewHTTPClient creates http.Client with optional SOCKS5 proxy and/or IPv6-first dial.
func NewHTTPClient(cfg *config.Config) (*http.Client, error) {
	resolver := newResolver()
	dialer := newDialer(resolver)

	// Без прокси — обычный клиент (с опциональным IPv6-first)
	if !cfg.Proxy.IsEnabled() {
		if cfg.Network.PreferIPv6 {
			d := &ipv6FirstDialer{next: dialer, resolver: resolver}
			return &http.Client{
				Transport: newTransport(d.DialContext),
				Timeout:   clientTimeout,
			}, nil
		}
		return &http.Client{
			Transport: newTransport(dialer.DialContext),
			Timeout:   clientTimeout,
		}, nil
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

	// Базовый диалектор для SOCKS5 — он подключается к самому прокси,
	// поэтому тоже обязан иметь таймаут на резолв и соединение.
	var base proxy.Dialer = dialer
	if cfg.Network.PreferIPv6 {
		base = &ipv6FirstDialer{next: dialer, resolver: resolver}
	}

	socksDialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, base)
	if err != nil {
		return nil, fmt.Errorf("proxy socks5 dialer: %w", err)
	}

	ctxDialer, ok := socksDialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("proxy socks5 dialer does not support context dialing")
	}

	return &http.Client{
		Transport: newTransport(ctxDialer.DialContext),
		Timeout:   clientTimeout,
	}, nil
}

// ipv6FirstDialer — обёртка, резолвит AAAA (IPv6) раньше A (IPv4).
type ipv6FirstDialer struct {
	next     proxy.Dialer
	resolver *net.Resolver
}

func (d *ipv6FirstDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *ipv6FirstDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return d.resolveAndDial(ctx, network, addr)
	}

	ips, err := d.resolver.LookupIPAddr(ctx, host)
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
		ctxDialer, ok := nextCtx.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("next dialer does not support context dialing")
		}
		conn, dialErr := ctxDialer.DialContext(ctx, network, target)
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
	ctxDialer, ok := nextCtx.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("next dialer does not support context dialing")
	}
	return ctxDialer.DialContext(ctx, network, addr)
}

var _ proxy.ContextDialer = (*ipv6FirstDialer)(nil)
