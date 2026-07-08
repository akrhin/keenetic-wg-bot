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
	"syscall"

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

	// Инициализация Telegram API (через прокси, если настроен)
	httpClient, err := proxy.NewHTTPClient(cfg)
	if err != nil {
		log.Fatalf("[wg-bot] proxy: %v", err)
	}
	api, err := tgbotapi.NewBotAPIWithClient(cfg.Telegram.BotToken, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		log.Fatalf("[wg-bot] telegram api: %v", err)
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

	log.Println("[wg-bot] stopped")
}
