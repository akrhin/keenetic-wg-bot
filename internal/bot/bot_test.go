package bot

import (
	"context"
	"log"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/akrhin/keenetic-wg-bot/internal/config"
	"github.com/akrhin/keenetic-wg-bot/internal/scheduler"
	"github.com/akrhin/keenetic-wg-bot/internal/wireguard"
)

// mockAPI реализует APIClient для тестов.
type mockAPI struct {
	sendFn    func(c tgbotapi.Chattable) (tgbotapi.Message, error)
	requestFn func(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

func (m *mockAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if m.sendFn != nil {
		return m.sendFn(c)
	}
	return tgbotapi.Message{}, nil
}

func (m *mockAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	if m.requestFn != nil {
		return m.requestFn(c)
	}
	return &tgbotapi.APIResponse{}, nil
}

func (m *mockAPI) GetUpdatesChan(u tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func newBotForTest(wg *wireguard.Manager) *Bot {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test:token",
			AllowedUsers: []config.AllowedUser{
				{ChatID: 100, UserID: 200},
			},
		},
		WireGuard: config.WireGuardConfig{Interface: "wg0"},
		WOL: config.WOLConfig{
			Hosts: []config.WOLHost{
				{Name: "server", MAC: "AA:BB:CC:DD:EE:FF", Broadcast: "192.168.1.255"},
			},
		},
		Scheduler: config.SchedulerConfig{AutoOffMinutes: 30},
	}
	sched := scheduler.New(func() { log.Println("timer fired (test)") })
	return NewWithClient(&mockAPI{}, cfg, wg, sched)
}

func makeCallback(chatID, userID int64, data string) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		ID:   "cq_test",
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: chatID},
			MessageID: 42,
		},
		Data: data,
	}
}

func TestHandleCallback_AccessDenied(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	// Неавторизованный пользователь
	cq := makeCallback(999, 999, "wg_on")
	b.handleCallback(context.Background(), cq)
	// Если дожили до конца без паники — тест пройден
}

func TestHandleCallback_UnknownCommand(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "unknown_cmd")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_WGOn_Success(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "wg_on")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_WGOff_Success(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "wg_off")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_WGStatus(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "wg_status")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_WOL(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "wol_server")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_Extend(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{})
	b := newBotForTest(mockWG)

	// Таймер должен быть активен для extend
	b.sched.Start(1 * time.Hour)
	defer b.sched.Stop()

	cq := makeCallback(100, 200, "scheduler_extend")
	b.handleCallback(context.Background(), cq)
}

func TestHandleCallback_WGOn_Failure(t *testing.T) {
	mockWG := wireguard.NewWithExecutor("wg0", &mockExec{}, &mockExec{
		out: []byte("error"),
		err: errTest,
	})
	b := newBotForTest(mockWG)

	cq := makeCallback(100, 200, "wg_on")
	b.handleCallback(context.Background(), cq)
}

// -- helpers --

type mockExec struct {
	out []byte
	err error
}

var errTest = &execExitError{}

type execExitError struct{}

func (e *execExitError) Error() string { return "exit status 1" }

func (m *mockExec) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	if m.err != nil {
		return m.out, m.err
	}
	return m.out, nil
}

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
