// Package wol — отправка Wake-on-LAN magic packet.
//
// Magic packet: 6 байт FF + 16 повторов MAC-адреса.
// Шлётся на broadcast-адрес UDP порт 9 (discard) или 7 (echo).
package wol

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// Send отправляет magic packet на указанный MAC и broadcast.
func Send(macStr, broadcastStr string) error {
	mac, err := parseMAC(macStr)
	if err != nil {
		return fmt.Errorf("wol: invalid MAC %q: %w", macStr, err)
	}

	// Magic packet: 6×0xFF + 16×MAC
	pkt := make([]byte, 6+16*6)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*6:], mac)
	}

	// Шлём через UDP на broadcast
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(broadcastStr, "9"))
	if err != nil {
		return fmt.Errorf("wol: resolve addr: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("wol: dial: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(pkt); err != nil {
		return fmt.Errorf("wol: write: %w", err)
	}

	// Шлём ещё на порт 7 для надёжности
	addr2, _ := net.ResolveUDPAddr("udp4", net.JoinHostPort(broadcastStr, "7"))
	conn2, _ := net.DialUDP("udp4", nil, addr2)
	if conn2 != nil {
		defer conn2.Close()
		_, _ = conn2.Write(pkt)
	}

	return nil
}

// parseMAC принимает MAC в любом формате (AA:BB:CC:DD:EE:FF, aa-bb-cc-dd-ee-ff, AABBCCDDEEFF).
func parseMAC(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	return hex.DecodeString(s)
}
