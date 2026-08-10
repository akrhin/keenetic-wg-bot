// Main — точка входа для wg-bot.
//
// Загружает конфиг, создаёт бота, запускает polling.
// Graceful shutdown по SIGINT/SIGTERM.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/akrhin/keenetic-wg-bot/internal/bot"
	"github.com/akrhin/keenetic-wg-bot/internal/config"
	"github.com/akrhin/keenetic-wg-bot/internal/proxy"
)

var configPath = flag.String("config", "/opt/etc/wg-bot/config.toml", "path to config file")

func main() {
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("[wg-bot] starting...")

	// Загрузка конфига
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[wg-bot] config: %v", err)
	}

	// Инициализация Telegram API (через прокси, если настроен).
	// При временном глюке сети (завис DNS/прокси) — ретраи вместо мгновенного падения.
	api, err := initAPI(cfg)
	if err != nil {
		log.Fatalf("[wg-bot] telegram api: %s", redactToken(cfg.Telegram.BotToken, err))
	}
	log.Printf("[wg-bot] authorized as %s", api.Self.UserName)

	// Создаём бота
	b := bot.New(api, cfg)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[wg-bot] received signal %v, shutting down...", sig)
		cancel()
	}()

	// Запуск
	if err := b.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("[wg-bot] run: %v", err)
	}

	// Graceful shutdown: останавливаем таймер и опускаем WG
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	b.Shutdown(shutdownCtx)

	log.Println("[wg-bot] stopped")
}

// initAPI создаёт Telegram-клиент с ретраями на старте.
// Ошибки API содержат URL с токеном — логируем через redactToken.
func initAPI(cfg *config.Config) (*tgbotapi.BotAPI, error) {
	attempts := []time.Duration{3 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}

	for i := 0; ; i++ {
		httpClient, herr := proxy.NewHTTPClient(cfg)
		if herr != nil {
			log.Printf("[wg-bot] proxy init failed (%d): %v", i+1, herr)
			if i >= len(attempts)-1 {
				return nil, herr
			}
			time.Sleep(attempts[i])
			continue
		}

		api, aerr := tgbotapi.NewBotAPIWithClient(cfg.Telegram.BotToken, tgbotapi.APIEndpoint, httpClient)
		if aerr == nil {
			return api, nil
		}
		log.Printf("[wg-bot] telegram api init failed (%d): %s", i+1, redactToken(cfg.Telegram.BotToken, aerr))
		if i >= len(attempts)-1 {
			return nil, aerr
		}
		time.Sleep(attempts[i])
	}
}

// redactToken маскирует токен бота в тексте ошибки.
// Без этого токен утекал бы в логи при каждом сбое getMe/getUpdates.
func redactToken(token string, err error) string {
	if err == nil {
		return ""
	}
	if token == "" {
		return err.Error()
	}
	return strings.ReplaceAll(err.Error(), token, "***")
}
