package config

import (
	"os"
	"testing"
)

func TestLoad_Minimal(t *testing.T) {
	content := `
[telegram]
bot_token = "test:token"

[[telegram.allowed_users]]
chat_id = 123
user_id = 456

[wireguard]
interface = "wg0"

[wol]

[scheduler]
auto_off_minutes = 30
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Telegram.BotToken != "test:token" {
		t.Errorf("bot_token = %q, want %q", cfg.Telegram.BotToken, "test:token")
	}
	if len(cfg.Telegram.AllowedUsers) != 1 {
		t.Fatalf("expected 1 allowed user, got %d", len(cfg.Telegram.AllowedUsers))
	}
	if cfg.Telegram.AllowedUsers[0].ChatID != 123 {
		t.Errorf("chat_id = %d, want 123", cfg.Telegram.AllowedUsers[0].ChatID)
	}
	if cfg.WireGuard.Interface != "wg0" {
		t.Errorf("interface = %q, want wg0", cfg.WireGuard.Interface)
	}
	if cfg.Scheduler.AutoOffMinutes != 30 {
		t.Errorf("auto_off_minutes = %d, want 30", cfg.Scheduler.AutoOffMinutes)
	}
}

func TestLoad_InvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestLoad_CommandTimeout_Default(t *testing.T) {
	content := `
[telegram]
bot_token = "test:token"

[[telegram.allowed_users]]
chat_id = 123
user_id = 456

[wireguard]
interface = "wg0"

[wol]

[scheduler]
auto_off_minutes = 30
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CommandTimeout != 45 {
		t.Errorf("CommandTimeout = %d, want 45", cfg.CommandTimeout)
	}
}

func TestLoad_CommandTimeout_Custom(t *testing.T) {
	content := `
command_timeout = 60

[telegram]
bot_token = "test:token"

[[telegram.allowed_users]]
chat_id = 123
user_id = 456

[wireguard]
interface = "wg0"

[wol]

[scheduler]
auto_off_minutes = 30
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CommandTimeout != 60 {
		t.Errorf("CommandTimeout = %d, want 60", cfg.CommandTimeout)
	}
}

func TestLoad_WOL_InvalidBroadcast(t *testing.T) {
	content := `
[telegram]
bot_token = "test:token"

[[telegram.allowed_users]]
chat_id = 123
user_id = 456

[wireguard]
interface = "wg0"

[[wol.hosts]]
name = "server"
mac = "AA:BB:CC:DD:EE:FF"
broadcast = "not-an-ip"
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid broadcast IP")
	}
}

func TestLoad_WOL_EmptyBroadcast(t *testing.T) {
	content := `
[telegram]
bot_token = "test:token"

[[telegram.allowed_users]]
chat_id = 123
user_id = 456

[wireguard]
interface = "wg0"

[[wol.hosts]]
name = "server"
mac = "AA:BB:CC:DD:EE:FF"
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.WOL.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.WOL.Hosts))
	}
	if cfg.WOL.Hosts[0].Broadcast != "" {
		t.Errorf("expected empty broadcast, got %q", cfg.WOL.Hosts[0].Broadcast)
	}
}

func TestLoad_EmptyToken(t *testing.T) {
	content := `
[telegram]

[[telegram.allowed_users]]
chat_id = 123
user_id = 456
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestLoad_NoUsers(t *testing.T) {
	content := `
[telegram]
bot_token = "test:token"
`
	path := writeTemp(t, content)
	defer func() { _ = os.Remove(path) }()

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for no allowed users")
	}
}

func TestIsAllowed(t *testing.T) {
	tg := TelegramConfig{
		AllowedUsers: []AllowedUser{
			{ChatID: 100, UserID: 200},
		},
	}

	tests := []struct {
		chatID int64
		userID int64
		want   bool
	}{
		{100, 200, true},
		{100, 999, false},
		{999, 200, false},
		{0, 0, false},
	}

	for _, tt := range tests {
		got := tg.IsAllowed(tt.chatID, tt.userID)
		if got != tt.want {
			t.Errorf("IsAllowed(%d,%d) = %v, want %v", tt.chatID, tt.userID, got, tt.want)
		}
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "wg-bot-test-*.toml")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()
	return f.Name()
}
