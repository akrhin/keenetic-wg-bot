package wol

import (
	"testing"
)

func TestParseMAC(t *testing.T) {
	tests := []struct {
		input string
		want  string // hex
	}{
		{"AA:BB:CC:DD:EE:FF", "aabbccddeeff"},
		{"aa-bb-cc-dd-ee-ff", "aabbccddeeff"},
		{"AABBCCDDEEFF", "aabbccddeeff"},
		{"aa:bb:cc:dd:ee:ff", "aabbccddeeff"},
		{"00:11:22:33:44:55", "001122334455"},
	}

	for _, tt := range tests {
		got, err := parseMAC(tt.input)
		if err != nil {
			t.Errorf("parseMAC(%q): unexpected error: %v", tt.input, err)
			continue
		}
		gotHex := ""
		for _, b := range got {
			gotHex += byteToHex(b)
		}
		if gotHex != tt.want {
			t.Errorf("parseMAC(%q) = %s, want %s", tt.input, gotHex, tt.want)
		}
	}
}

func TestSend_Localhost(t *testing.T) {
	// Шлём на localhost:9999 — UDP без соединения, пакет просто улетит.
	// Ошибка может быть только если система не даёт открыть UDP-сокет.
	err := Send("AA:BB:CC:DD:EE:FF", "127.0.0.255")
	if err != nil {
		t.Fatalf("Send to localhost failed: %v", err)
	}
}

func TestSend_InvalidMAC(t *testing.T) {
	err := Send("invalid", "127.0.0.255")
	if err == nil {
		t.Fatal("expected error for invalid MAC")
	}
}

func byteToHex(b byte) string {
	hex := "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}
