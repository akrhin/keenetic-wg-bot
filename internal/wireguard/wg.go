// Package wireguard — управление WireGuard-интерфейсом.
//
// Использует wg-quick для up/down и wg show для статуса.
// Все вызовы — через exec.Command (без shell), никаких инъекций.
package wireguard

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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
	iface string
}

// New создаёт Manager для указанного интерфейса.
func New(iface string) *Manager {
	return &Manager{iface: iface}
}

// Up поднимает интерфейс через wg-quick up.
func (m *Manager) Up(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "wg-quick", "up", m.iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick up %s: %w\n%s", m.iface, err, string(out))
	}
	return nil
}

// Down опускает интерфейс через wg-quick down.
func (m *Manager) Down(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "wg-quick", "down", m.iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down %s: %w\n%s", m.iface, err, string(out))
	}
	return nil
}

// Show возвращает статус интерфейса через wg show.
func (m *Manager) Show(ctx context.Context) (*Status, error) {
	cmd := exec.CommandContext(ctx, "wg", "show", m.iface, "dump")
	out, err := cmd.Output()
	if err != nil {
		// Если интерфейс не существует — это не ошибка, просто не запущен.
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			// wg show пишет ошибку в stderr — это нормально, когда интерфейса нет.
			return &Status{Running: false, Interface: m.iface}, nil
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
	fmt.Sscanf(fields[2], "%d", &s.ListenPort)

	return s, nil
}
