package wireguard

import (
	"testing"
)

func TestParseDump_Running(t *testing.T) {
	dump := "privateKey\tpublicKey\t51820\tfirewall\t1234"
	s, err := parseDump("wg0", dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Running {
		t.Error("expected running=true")
	}
	if s.Interface != "wg0" {
		t.Errorf("interface = %q, want wg0", s.Interface)
	}
	if s.ListenPort != 51820 {
		t.Errorf("listen_port = %d, want 51820", s.ListenPort)
	}
	if s.PeerCount != 0 {
		t.Errorf("peers = %d, want 0", s.PeerCount)
	}
}

func TestParseDump_WithPeers(t *testing.T) {
	dump := "privateKey\tpublicKey\t0\t\npeerKey\tendpoint\t0\t0\t0\t0\t0\npeerKey2\tendpoint2\t0\t0\t0\t0\t0"
	s, err := parseDump("wg0", dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Running {
		t.Error("expected running=true")
	}
	if s.PeerCount != 2 {
		t.Errorf("peers = %d, want 2", s.PeerCount)
	}
}

func TestParseDump_NotRunning(t *testing.T) {
	s, err := parseDump("wg0", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Running {
		t.Error("expected running=false for empty dump")
	}
}
