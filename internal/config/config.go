// Package config — загрузка и валидация TOML-конфига.
//
// Конфиг лежит в /opt/etc/wg-bot/config.toml (chmod 600).
// Токены и ключи хранятся ТОЛЬКО в этом файле, вне git.
package config

import (
	"fmt"
	"net"
	"os"

	"github.com/BurntSushi/toml"
)

// Config — корневая структура конфигурации.
type Config struct {
	Telegram       TelegramConfig  `toml:"telegram"`
	WireGuard      WireGuardConfig `toml:"wireguard"`
	WOL            WOLConfig       `toml:"wol"`
	Scheduler      SchedulerConfig `toml:"scheduler"`
	Network        NetworkConfig   `toml:"network"`
	Proxy          ProxyConfig     `toml:"proxy"`
	CommandTimeout int             `toml:"command_timeout"`
}

type NetworkConfig struct {
	PreferIPv6 bool `toml:"prefer_ipv6"`
}

type TelegramConfig struct {
	BotToken string `toml:"bot_token"`
	// AllowedUsers — список пар chat_id + user_id, которым бот отвечает.
	AllowedUsers []AllowedUser `toml:"allowed_users"`
}

type AllowedUser struct {
	ChatID int64 `toml:"chat_id"`
	UserID int64 `toml:"user_id"`
}

type WireGuardConfig struct {
	Interface  string `toml:"interface"`
	ConfigPath string `toml:"config_path"`
}

type WOLConfig struct {
	Hosts []WOLHost `toml:"hosts"`
}

type WOLHost struct {
	Name      string `toml:"name"`
	MAC       string `toml:"mac"`
	Broadcast string `toml:"broadcast"`
}

type SchedulerConfig struct {
	AutoOffMinutes int `toml:"auto_off_minutes"`
}

// ProxyConfig — прокси для Telegram API (SOCKS5 с авторизацией или без).
type ProxyConfig struct {
	Enabled  bool   `toml:"enabled"`
	URL      string `toml:"url"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// Enabled возвращает true, если прокси включён и настроен.
func (p *ProxyConfig) IsEnabled() bool {
	return p.Enabled && p.URL != ""
}

// Load читает и валидирует конфиг.
func Load(path string) (*Config, error) {
	var cfg Config

	// #nosec G304 — path comes from -config flag with a default, not user input
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config read: %w", err)
	}

	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}

	if cfg.Telegram.BotToken == "" {
		return nil, fmt.Errorf("config: telegram.bot_token is required")
	}
	if len(cfg.Telegram.AllowedUsers) == 0 {
		return nil, fmt.Errorf("config: at least one telegram.allowed_users entry is required")
	}
	if cfg.WireGuard.Interface == "" {
		cfg.WireGuard.Interface = "wg0"
	}
	if cfg.WireGuard.ConfigPath == "" {
		cfg.WireGuard.ConfigPath = "/opt/etc/wireguard/" + cfg.WireGuard.Interface + ".conf"
	}
	if cfg.Scheduler.AutoOffMinutes == 0 {
		cfg.Scheduler.AutoOffMinutes = 30
	}
	if cfg.CommandTimeout <= 0 {
		cfg.CommandTimeout = 45
	}

	for _, h := range cfg.WOL.Hosts {
		if h.Name == "" || h.MAC == "" {
			return nil, fmt.Errorf("config: wol.host requires name and mac")
		}
		if h.Broadcast != "" {
			if net.ParseIP(h.Broadcast) == nil {
				return nil, fmt.Errorf("config: wol.host %q: invalid broadcast IP %q", h.Name, h.Broadcast)
			}
		}
	}

	return &cfg, nil
}

// IsAllowed проверяет, разрешён ли пользователь.
func (t *TelegramConfig) IsAllowed(chatID, userID int64) bool {
	for _, u := range t.AllowedUsers {
		if u.ChatID == chatID && u.UserID == userID {
			return true
		}
	}
	return false
}
