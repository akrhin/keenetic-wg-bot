package wireguard

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// mockExec реализует Executor для тестов.
type mockExec struct {
	out   []byte
	err   error
	stder []byte // имитация stderr в ExitError
}

func (m *mockExec) Run(_ context.Context, name string, arg ...string) ([]byte, error) {
	if m.err != nil {
		// CombinedOutput semantics: stderr is merged into out, exitErr.Stderr is empty
		if m.stder != nil {
			merged := append(m.out, m.stder...)
			exitErr := &exec.ExitError{Stderr: m.stder}
			return merged, exitErr
		}
		return m.out, m.err
	}
	return m.out, nil
}

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

func TestShow_Success(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{out: []byte("key	key	51820		1234")},
	)
	s, err := mgr.Show(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Running {
		t.Error("expected running=true")
	}
}

func TestShow_DeviceNotFound(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{
			out:   []byte{},
			err:   errors.New("exit status 1"),
			stder: []byte("Cannot find device wg0"),
		},
	)
	s, err := mgr.Show(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Running {
		t.Error("expected running=false for missing device")
	}
}

func TestUp_Success(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{out: []byte("ok")},
	)
	err := mgr.Up(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUp_Failure(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{
			out:   []byte("error"),
			err:   errors.New("exit status 1"),
			stder: []byte("wg-quick: error"),
		},
	)
	err := mgr.Up(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDown_Success(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{out: []byte("ok")},
	)
	err := mgr.Down(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDown_Failure(t *testing.T) {
	mgr := NewWithExecutor("wg0",
		&mockExec{},
		&mockExec{
			err: errors.New("exit status 1"),
		},
	)
	err := mgr.Down(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
