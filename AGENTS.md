# AGENTS.md — Контекст для LLM

## Проект

`keenetic-wg-bot` — Telegram-бот на Go для Keenetic с Entware.
Управляет WireGuard-клиентом (вкл/выкл через wg-quick) и WoL.

## Ключевые решения

| Решение | Значение | Причина |
|---------|----------|---------|
| Язык | Go | Статика, кросс-компиляция одной строкой, ноль зависимостей на роутере |
| Цель | MIPSEL soft-float | Entware на Keenetic — `mipsel-3.4` |
| GOARCH | `mipsle` + `GOMIPS=softfloat` | Аппаратное FPU отсутствует |
| Telegram API | `go-telegram-bot-api/v5` | Стандарт, pure Go |
| Конфиг | TOML | Человекочитаемый, с комментариями, строгая типизация |
| WoL | `net.DialUDP` → broadcast | Чистый сокет, без extern-пакетов |
| WG | `os/exec` → `wg-quick up/down` | shell=false — без инъекций |
| Таймер | `time.AfterFunc` | Живёт внутри процесса, `at` не нужен |
| Сборка | GitHub Actions | Lint + gosec + gitleaks + test + build + release |

## Структура

```
cmd/wg-bot/main.go              # Entry point + graceful shutdown
internal/
  bot/bot.go                    # Telegram handling + inline keyboard
  config/config.go              # TOML loading + validation
  wireguard/wg.go               # wg-quick up/down/show
  wol/wol.go                    # Magic packet via UDP
  scheduler/timer.go            # Auto-off timer
```

## Пайплайн CI

```yaml
push → lint (golangci) → gosec → gitleaks → test (-race) → build → [tag→release]
```

## Сборка (локально)

```bash
make all          # fmt → lint → test → build
make build        # amd64 + mipsle
make release      # test + build + tar.gz
```

## Статус

✅ Конфиг — загрузка + валидация + тесты
✅ WireGuard — up/down/show + парсинг dump + тесты
✅ WoL — magic packet + тесты
✅ Scheduler — таймер автовыключения + тесты
✅ Bot — Telegram-хендлеры + inline-кнопки
✅ Main — graceful shutdown
✅ Makefile
✅ CI (lint + test + build + release)
✅ install.sh
✅ Лицензия MIT
✅ README
✅ AGENTS.md

⏳ BGP/OPSF/EVPN — в параллельной задаче (docling conversion в фоне)
