// Package bot — Telegram-бот с inline-кнопками.
//
// Никаких команд, только callback_data.
// Для отладки /status и /help работают как текст.
package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/akrhin/keenetic-wg-bot/internal/config"
	"github.com/akrhin/keenetic-wg-bot/internal/scheduler"
	"github.com/akrhin/keenetic-wg-bot/internal/wireguard"
	"github.com/akrhin/keenetic-wg-bot/internal/wol"
)

// APIClient — интерфейс для Telegram API (для тестирования).
type APIClient interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(u tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}

// Bot — основной объект бота.
type Bot struct {
	api   APIClient
	cfg   *config.Config
	wg    *wireguard.Manager
	sched *scheduler.Timer
}

// New создаёт нового бота.
func New(api *tgbotapi.BotAPI, cfg *config.Config) *Bot {
	wgMgr := wireguard.New(cfg.WireGuard.Interface)

	b := &Bot{
		api:   api,
		cfg:   cfg,
		wg:    wgMgr,
	}

	// Таймер автоотключения
	b.sched = scheduler.New(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := b.wg.Down(ctx); err != nil {
			log.Printf("[bot] auto-off: wg down failed: %v", err)
			return
		}

		// Уведомляем пользователей
		msg := "⏰ Автоотключение: WireGuard выключен"
		for _, u := range cfg.Telegram.AllowedUsers {
			m := tgbotapi.NewMessage(u.ChatID, msg)
			if _, err := b.api.Send(m); err != nil {
				log.Printf("[bot] notify auto-off: %v", err)
			}
		}
		log.Println("[bot] auto-off: wireguard stopped by timer")
	})

	return b
}

// NewWithClient создаёт бота с произвольным APIClient (для тестов).
func NewWithClient(api APIClient, cfg *config.Config, wg *wireguard.Manager, sched *scheduler.Timer) *Bot {
	return &Bot{
		api:   api,
		cfg:   cfg,
		wg:    wg,
		sched: sched,
	}
}

// Run запускает бесконечный цикл polling'а.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			b.handleUpdate(ctx, update)
		}
	}
}

// Shutdown выполняет graceful shutdown: останавливает таймер и опускает WG.
func (b *Bot) Shutdown(ctx context.Context) {
	log.Println("[bot] shutting down...")
	if b.sched != nil {
		b.sched.Stop()
	}
	if err := b.wg.Down(ctx); err != nil {
		log.Printf("[bot] wg down on shutdown: %v", err)
	}
	log.Println("[bot] shutdown complete")
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Callback — нажатие кнопки
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update.CallbackQuery)
		return
	}

	// Текстовое сообщение
	if update.Message == nil {
		return
	}

	// Проверка доступа
	if !b.cfg.Telegram.IsAllowed(update.Message.Chat.ID, update.Message.From.ID) {
		log.Printf("[bot] access denied: chat=%d user=%d", update.Message.Chat.ID, update.Message.From.ID)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Доступ запрещён")
		_, _ = b.api.Send(msg)
		return
	}

	switch {
	case update.Message.IsCommand() && update.Message.Command() == "start":
		b.sendMainMenu(update.Message.Chat.ID)
	case update.Message.IsCommand() && update.Message.Command() == "status":
		b.sendStatus(update.Message.Chat.ID)
	case strings.EqualFold(update.Message.Text, "статус"):
		b.sendStatus(update.Message.Chat.ID)
	default:
		b.sendMainMenu(update.Message.Chat.ID)
	}
}

// cb — короткий вызов callback answer (wraps Request).
func (b *Bot) cb(cq *tgbotapi.CallbackQuery, text string) {
	_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, text))
}

func (b *Bot) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	log.Printf("[bot] callback: data=%s from=%d chat=%d", cq.Data, cq.From.ID, cq.Message.Chat.ID)

	// Проверка доступа
	if !b.cfg.Telegram.IsAllowed(cq.Message.Chat.ID, cq.From.ID) {
		b.cb(cq, "⛔ Доступ запрещён")
		return
	}

	// Используем командный таймаут из конфига
	timeout := time.Duration(b.cfg.CommandTimeout) * time.Second
	cmdCtx, cmdCancel := context.WithTimeout(ctx, timeout)
	defer cmdCancel()

	switch cq.Data {
	case "wg_on":
		b.cmdOn(cq, cmdCtx)
	case "wg_off":
		b.cmdOff(cq, cmdCtx)
	case "wg_status":
		b.cmdStatus(cq, cmdCtx)
	case "scheduler_extend":
		b.cmdExtend(cq, cmdCtx)
	case "wol_server":
		b.cmdWOL(cq, cmdCtx)
	default:
		b.cb(cq, "❓ Неизвестная команда")
	}
}

func (b *Bot) sendMainMenu(chatID int64) {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Включить", "wg_on"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Выключить", "wg_off"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Статус", "wg_status"),
			tgbotapi.NewInlineKeyboardButtonData("⏱ Продлить", "scheduler_extend"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ WoL сервер", "wol_server"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "🔐 **Управление VPN**\n\nВыбери действие:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = kb
	msg.DisableWebPagePreview = true

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("[bot] send menu: %v", err)
	}
}

func (b *Bot) sendStatus(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := b.wg.Show(ctx)
	text := formatStatus(status, err, b.sched)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true

	_, _ = b.api.Send(msg)
}

func (b *Bot) cmdOn(cq *tgbotapi.CallbackQuery, ctx context.Context) {
	if err := b.wg.Up(ctx); err != nil {
		b.cb(cq, "❌ Ошибка включения")
		log.Printf("[bot] wg up: %v", err)
		return
	}

	// Запуск таймера
	d := time.Duration(b.cfg.Scheduler.AutoOffMinutes) * time.Minute
	b.sched.Start(d)

	log.Printf("[bot] wg on: interface=%s timeout=%dm", b.cfg.WireGuard.Interface, b.cfg.Scheduler.AutoOffMinutes)
	b.cb(cq, "✅ WG включён")

	// Обновляем сообщение со статусом
	status, err := b.wg.Show(ctx)
	text := "✅ **WireGuard включён**\n\n" + formatStatus(status, err, b.sched)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = cq.Message.ReplyMarkup
	_, _ = b.api.Send(edit)
}

func (b *Bot) cmdOff(cq *tgbotapi.CallbackQuery, ctx context.Context) {
	b.sched.Stop()

	if err := b.wg.Down(ctx); err != nil {
		b.cb(cq, "❌ Ошибка выключения")
		log.Printf("[bot] wg down: %v", err)
		return
	}

	log.Printf("[bot] wg off: interface=%s", b.cfg.WireGuard.Interface)
	b.cb(cq, "✅ WG выключен")

	status, err := b.wg.Show(ctx)
	text := "❌ **WireGuard выключен**\n\n" + formatStatus(status, err, b.sched)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = cq.Message.ReplyMarkup
	_, _ = b.api.Send(edit)
}

func (b *Bot) cmdStatus(cq *tgbotapi.CallbackQuery, ctx context.Context) {
	b.cb(cq, "")

	status, err := b.wg.Show(ctx)
	text := formatStatus(status, err, b.sched)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = cq.Message.ReplyMarkup
	_, _ = b.api.Send(edit)
}

func (b *Bot) cmdExtend(cq *tgbotapi.CallbackQuery, ctx context.Context) {
	if !b.sched.IsActive() {
		b.cb(cq, "❌ Таймер не активен")

		status, err := b.wg.Show(ctx)
		text := formatStatus(status, err, b.sched)
		edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = cq.Message.ReplyMarkup
		_, _ = b.api.Send(edit)
		return
	}

	d := time.Duration(b.cfg.Scheduler.AutoOffMinutes) * time.Minute
	b.sched.Start(d)

	s := b.sched.Remaining()
	log.Printf("[bot] extend: remaining=%v", s)
	b.cb(cq, fmt.Sprintf("⏱ Продлён на %d мин", b.cfg.Scheduler.AutoOffMinutes))

	status, err := b.wg.Show(ctx)
	text := formatStatus(status, err, b.sched)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = cq.Message.ReplyMarkup
	_, _ = b.api.Send(edit)
}

func (b *Bot) cmdWOL(cq *tgbotapi.CallbackQuery, ctx context.Context) {
	if len(b.cfg.WOL.Hosts) == 0 {
		b.cb(cq, "❌ Нет хостов для WoL")
		return
	}

	host := b.cfg.WOL.Hosts[0]
	if err := wol.Send(host.MAC, host.Broadcast); err != nil {
		b.cb(cq, "❌ Ошибка WoL")
		log.Printf("[bot] wol: %v", err)
		return
	}

	log.Printf("[bot] wol sent: name=%s mac=%s broadcast=%s", host.Name, host.MAC, host.Broadcast)
	b.cb(cq, fmt.Sprintf("⚡ WoL отправлен на %s", host.Name))
}

// formatStatus форматирует статус WG для сообщения.
func formatStatus(s *wireguard.Status, err error, t *scheduler.Timer) string {
	if err != nil {
		return fmt.Sprintf("❌ Ошибка получения статуса:\n`%v`", err)
	}

	var b strings.Builder
	if s.Running {
		b.WriteString("✅ **WireGuard запущен**\n")
	} else {
		b.WriteString("⛔ **WireGuard остановлен**\n")
		b.WriteString("\n_Нет подключения к VPN_\n")
		if timerInfo := t.String(); timerInfo != "not running" {
			b.WriteString("\n⏱ " + timerInfo + "\n")
		}
		return b.String()
	}

	fmt.Fprintf(&b, "🔌 **Интерфейс:** `%s`\n", s.Interface)
	if s.ListenPort > 0 {
		fmt.Fprintf(&b, "🔢 **Порт:** %d\n", s.ListenPort)
	}
	fmt.Fprintf(&b, "👥 **Пиров:** %d\n", s.PeerCount)

	if timerInfo := t.String(); timerInfo != "not running" {
		b.WriteString("\n⏱ " + timerInfo + "\n")
	}

	return b.String()
}
