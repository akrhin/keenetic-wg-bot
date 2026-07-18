# AGENTS.md — Контекст для LLM

## Проект

`keenetic-wg-bot` — Telegram-бот на Go для Keenetic с Entware.
Управляет WireGuard-клиентом (вкл/выкл через wg-quick), WoL, таймером.

## Ключевые решения

| Решение | Значение | Причина |
|---------|----------|---------|
| Язык | Go | Статика, кросс-компиляция одной строкой, ноль зависимостей на роутере |
| Цель | MIPSEL soft-float | Entware на Keenetic — `mipsel-3.4` |
| GOARCH | `mipsle` + `GOMIPS=softfloat` | Аппаратное FPU отсутствует |
| Telegram API | `go-telegram-bot-api/v5` | Стандарт, pure Go |
| Конфиг | TOML | Человекочитаемый, с комментариями, строгая типизация |
| WoL | `net.DialUDP` → broadcast (UDP 9 + 7) | Чистый сокет, без extern-пакетов |
| WG | `exec.Command` → `wg-quick up/down`, `wg show dump` | shell=false — без инъекций |
| Таймер | `time.AfterFunc` + мьютекс | Живёт внутри процесса |
| Прокси | `golang.org/x/net/proxy` | SOCKS5 из extended stdlib |
| Интерфейс | Inline-клавиатура | Без текстовых команд, callback_data |
| Сборка | Make + GitHub Actions | Lint + gosec + gitleaks + govulncheck + test + build + release |

## Структура

```
cmd/wg-bot/main.go              # Entry point + graceful shutdown
internal/
  bot/bot.go                    # Telegram handling + inline keyboard
  config/config.go              # TOML loading + validation + whitelist
  proxy/proxy.go                # SOCKS5 + IPv6-first dialer
  wireguard/wg.go               # wg-quick up/down, wg show dump parsing
  wol/wol.go                    # Magic packet via UDP (port 9 + 7)
  scheduler/timer.go            # Auto-off timer (time.AfterFunc)
scripts/wg-bot.sh               # Management script (start/stop/restart/status)
install.sh                      # Entware installer (idempotent)
```

## Пайплайн CI

```yaml
push → lint (golangci) → gosec → gitleaks → vet → govulncheck
                                                     ↓
                                              test (-race)
                                                     ↓
                                              build → [tag→release]
```

## Сборка (локально)

```bash
make all          # fmt → lint → security → test → build
make build        # amd64 + mipsle
make release      # test + build + tar.gz
```

## Статус

☑️ Конфиг — загрузка + валидация + тесты
☑️ WireGuard — up/down/show + парсинг dump + тесты
☑️ WoL — magic packet (UDP 9 + 7) + тесты
☑️ Proxy — SOCKS5 с/без авторизации + IPv6-first
☑️ Scheduler — таймер автовыключения + тесты
☑️ Bot — Telegram-хендлеры + inline-кнопки
☑️ Main — graceful shutdown
☑️ Makefile
☑️ CI (lint + gosec + gitleaks + vet + govulncheck + test + build + release)
☑️ install.sh (Entware, идемпотентный)
☑️ scripts/wg-bot.sh (start/stop/restart/status/logs/enable/disable)
☑️ Лицензия MIT
☑️ README
☑️ AGENTS.md
☑️ ARCHITECTURE.md
