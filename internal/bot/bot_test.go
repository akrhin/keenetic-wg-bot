// Package bot — tests for Telegram bot logic (white-box).
package bot

import (
	"testing"
	"time"

	"github.com/akrhin/keenetic-wg-bot/internal/scheduler"
	"github.com/akrhin/keenetic-wg-bot/internal/wireguard"
)

func TestFormatStatus_Err(t *testing.T) {
	got := formatStatus(nil, assertError("test error"), nil)
	if !contains(got, "Ошибка получения статуса") {
		t.Errorf("expected error, got: %s", got)
	}
}

func TestFormatStatus_Running(t *testing.T) {
	s := &wireguard.Status{Running: true, Interface: "wg0", ListenPort: 51820, PeerCount: 3}
	tmr := scheduler.New(func() {})
	got := formatStatus(s, nil, tmr)
	if !contains(got, "WireGuard запущен") {
		t.Errorf("expected running, got: %s", got)
	}
	if !contains(got, "wg0") {
		t.Errorf("expected wg0, got: %s", got)
	}
	if !contains(got, "51820") {
		t.Errorf("expected port, got: %s", got)
	}
}

func TestFormatStatus_RunningWithTimer(t *testing.T) {
	s := &wireguard.Status{Running: true, Interface: "wg1", PeerCount: 0}
	tmr := scheduler.New(func() {})
	tmr.Start(25 * time.Minute)
	got := formatStatus(s, nil, tmr)
	if !contains(got, "WireGuard запущен") {
		t.Errorf("expected running, got: %s", got)
	}
}

func TestFormatStatus_Stopped(t *testing.T) {
	s := &wireguard.Status{Running: false}
	tmr := scheduler.New(func() {})
	got := formatStatus(s, nil, tmr)
	if !contains(got, "WireGuard остановлен") {
		t.Errorf("expected stopped, got: %s", got)
	}
	if !contains(got, "Нет подключения") {
		t.Errorf("expected no-connection, got: %s", got)
	}
}

func TestFormatStatus_StoppedWithTimer(t *testing.T) {
	s := &wireguard.Status{Running: false}
	tmr := scheduler.New(func() {})
	tmr.Start(10 * time.Minute)
	got := formatStatus(s, nil, tmr)
	if !contains(got, "WireGuard остановлен") {
		t.Errorf("expected stopped, got: %s", got)
	}
}

func TestFormatStatus_RunningNoTimer(t *testing.T) {
	s := &wireguard.Status{Running: true, Interface: "wg99"}
	tmr := scheduler.New(func() {})
	got := formatStatus(s, nil, tmr)
	if !contains(got, "wg99") {
		t.Errorf("expected wg99, got: %s", got)
	}
	// Timer not active — no timer line expected
}

// ── helpers ─────────────────────────────────────────────────────────────┘

func assertError(msg string) error {
	return &simpleError{msg: msg}
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsBrute(s, substr)
}

func containsBrute(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
