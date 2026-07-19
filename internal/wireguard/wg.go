// Package wireguard — управление WireGuard-интерфейсом.
//
// Использует wg-quick для up/down и wg show для статуса.
// Все вызовы — через exec.Command (без shell), никаких инъекций.
//
// Testing: передайте mock Executor в NewWithExecutor для изоляции.
package wireguard

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Executor — интерфейс для выполнения команд wg/wg-quick.
// В продакшене — RealExecutor (os/exec). В тестах — MockExecutor.
type Executor interface {
	Run(ctx context.Context, name string, arg ...string) ([]byte, error)
}

// RealExecutor выполняет команды через os/exec.
type RealExecutor struct{}

func (e RealExecutor) Run(ctx context.Context, name string, arg ...string) ([]byte, error) {
	// #nosec G204 — name/arg from local config, not user input
	return exec.CommandContext(ctx, name, arg...).Output()
}

// CombinedExec выполняет команду и возвращает объединённый stdout+stderr.
type CombinedExec struct{}

func (e CombinedExec) Run(ctx context.Context, name string, arg ...string) ([]byte, error) {
	// #nosec G204 — name/arg from local config, not user input
	return exec.CommandContext(ctx, name, arg...).CombinedOutput()
}

// Status содержит текущее состояние WG-интерфейса.
type Status struct {
	Running    bool      `json:"running"`
	Interface  string    `json:"interface"`
	PublicKey  string    `json:"public_key,omitempty"`
	ListenPort int       `json:"listen_port,omitempty"`
	PeerCount  int       `json:"peer_count"`
	LatestRX   int64     `json:"latest_rx,omitempty"`
	LatestTX   int64     `json:"latest_tx,omitempty"`
	LastOK     time.Time `json:"last_ok,omitempty"`
}

// Manager управляет WG-интерфейсом.
type Manager struct {
	iface    string
	exec     Executor
	combExec Executor // для wg-quick (CombinedOutput)
}

// New создаёт Manager для указанного интерфейса.
func New(iface string) *Manager {
	return &Manager{
		iface:    iface,
		exec:     RealExecutor{},
		combExec: CombinedExec{},
	}
}

// NewWithExecutor создаёт Manager с указанным Executor (для тестов).
func NewWithExecutor(iface string, exec Executor, combExec Executor) *Manager {
	return &Manager{
		iface:    iface,
		exec:     exec,
		combExec: combExec,
	}
}

// Up поднимает интерфейс через wg-quick up.
func (m *Manager) Up(ctx context.Context) error {
	out, err := m.combExec.Run(ctx, "wg-quick", "up", m.iface)
	if err != nil {
		return fmt.Errorf("wg-quick up %s: %w\n%s", m.iface, err, string(out))
	}
	return nil
}

// Down опускает интерфейс через wg-quick down.
func (m *Manager) Down(ctx context.Context) error {
	out, err := m.combExec.Run(ctx, "wg-quick", "down", m.iface)
	if err != nil {
		return fmt.Errorf("wg-quick down %s: %w\n%s", m.iface, err, string(out))
	}
	return nil
}

// Show возвращает статус интерфейса через wg show.
func (m *Manager) Show(ctx context.Context) (*Status, error) {
	out, err := m.exec.Run(ctx, "wg", "show", m.iface, "dump")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "Cannot find") || strings.Contains(stderr, "does not exist") {
				return &Status{Running: false, Interface: m.iface}, nil
			}
		}
		return nil, fmt.Errorf("wg show %s: %w", m.iface, err)
	}
	return parseDump(m.iface, string(out))
}

// parseDump разбирает вывод `wg show <iface> dump`.
// Формат: private_key\tpublic_key\tlisten_port\t...
// Затем по одной строке на peer.
func parseDump(iface, dump string) (*Status, error) {
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return &Status{Running: false, Interface: iface}, nil
	}

	fields := strings.Split(lines[0], "\t")
	if len(fields) < 3 {
		return &Status{Running: false, Interface: iface}, nil
	}

	s := &Status{
		Running:   true,
		Interface: iface,
		PeerCount: len(lines) - 1,
	}

	// Парсим порт
	_, _ = fmt.Sscanf(fields[2], "%d", &s.ListenPort)

	return s, nil
}
