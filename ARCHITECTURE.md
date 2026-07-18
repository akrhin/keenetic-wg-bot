# Keenetic WireGuard Bot — Архитектура

## Overview

`keenetic-wg-bot` — Telegram-бот на Go для роутеров Keenetic с Entware. Управляет WireGuard-клиентом (вкл/выкл через `wg-quick`), отправляет Wake-on-LAN на хосты в локальной сети и автоматически отключает VPN по таймеру.

Интерфейс — inline-клавиатура в Telegram, без текстовых команд. Бот проверяет доступ по белому списку `chat_id + user_id`.

## Технологический стек

| Компонент | Технология | Обоснование |
|-----------|------------|-------------|
| Язык | Go 1.23+ | Один бинарник, кросс-компиляция, ноль зависимостей на роутере |
| Telegram API | `go-telegram-bot-api/v5` | Pure Go, стандарт де-факто |
| Конфиг | TOML (`BurntSushi/toml`) | Человекочитаемый, с комментариями, строгая типизация |
| Прокси | `golang.org/x/net/proxy` | SOCKS5 из стандартного extended-пакета |
| Сборка | Make + GitHub Actions | Кросс-компиляция, линтинг, gosec, gitleaks, релизы |

## Структура проекта

```
keenetic-wg-bot/
├── cmd/wg-bot/main.go              # Entry point + graceful shutdown
├── internal/
│   ├── bot/bot.go                  # Telegram-хендлеры + inline-клавиатура
│   ├── config/config.go            # Загрузка TOML + валидация + белый список
│   ├── wireguard/wg.go             # exec.Command wg-quick, парсинг wg show dump
│   ├── wol/wol.go                  # Magic packet через UDP
│   ├── scheduler/timer.go          # time.AfterFunc + мьютекс + Remaining()
│   └── proxy/proxy.go              # SOCKS5 + IPv6-first dialer
├── install.sh                      # Идемпотентный установщик на Entware
├── config.toml.example             # Пример конфига (без секретов)
├── Makefile                        # fmt → lint → security → test → build → release
├── AGENTS.md                       # Контекст для LLM
├── ARCHITECTURE.md                 # Этот файл
├── README.md
└── .github/workflows/build.yml     # CI: lint+gosec+gitleaks → test → build → release
```

Каждый пакет имеет тесты (`_test.go`).

## Компоненты

### 1. Config (`internal/config`)

Загрузка и валидация TOML-конфига (`/opt/etc/wg-bot/config.toml`).

**Секции конфига:**
- `telegram` — bot_token, белый список `{chat_id, user_id}`
- `wireguard` — имя интерфейса (по умолчанию `wg0`), путь к `.conf`
- `wol` — список хостов `{name, mac, broadcast}`
- `scheduler` — `auto_off_minutes` (по умолчанию 30), `state_file`
- `proxy` — опциональный SOCKS5: `enabled, url, username, password`
- `network` — `prefer_ipv6` (IPv6-first resolution)

**Валидация:**
- `bot_token` обязателен
- `allowed_users` не может быть пустым
- Для каждого WoL-хоста обязательны `name` и `mac`
- `interface`, `config_path`, `auto_off_minutes` имеют значения по умолчанию

**Access control:** метод `IsAllowed(chatID, userID int64) bool` проверяет пару `chat_id + user_id` по белому списку.

### 2. WireGuard Manager (`internal/wireguard`)

Управление WG-интерфейсом через `exec.Command` **без shell** (`shell=false`).

| Действие | Вызов | Примечание |
|----------|-------|------------|
| Включить | `wg-quick up <iface>` | `exec.CommandContext(ctx, ...)` |
| Выключить | `wg-quick down <iface>` | `exec.CommandContext(ctx, ...)` |
| Статус | `wg show <iface> dump` | Парсинг tab-разделённого вывода |

**Парсинг `wg show dump`:**
- Первая строка — информация об интерфейсе (listen_port)
- Остальные строки — пиры (считаем `PeerCount`)
- Если интерфейс не существует — возвращается `{Running: false}`, а не ошибка
- Устойчив к BusyBox `wg` (Keenetic) и `wg-tools` (стандартный)

### 3. Scheduler Timer (`internal/scheduler`)

Внутрипроцессный таймер автоотключения на базе `time.AfterFunc`.

**Состояние:**
- `mu sync.Mutex` — защита от гонок
- `timer *time.Timer` — системный таймер
- `endAt time.Time` — время срабатывания (для живого `Remaining()`)
- `active bool` — флаг активности

**Методы:**
- `Start(d)` — запустить/перезапустить на `d` (старый таймер отменяется)
- `Stop()` — отменить таймер
- `IsActive() bool` — запущен ли
- `Remaining() time.Duration` — живой обратный отсчёт от `endAt`

**Колбэк** — action-функция, переданная в `New()`. Вызывает `wg.Down()` + уведомляет всех пользователей из белого списка.

Таймер живёт в ОЗУ процесса. `at`/`cron` не используются.

### 4. Wake-on-LAN (`internal/wol`)

Отправка magic packet через UDP-сокет.

**Формат пакета:** 6 байт `0xFF` + 16 повторов MAC-адреса (102 байта).

**Отправка:**
- Основной порт: UDP 9 (discard)
- Fallback: UDP 7 (echo) — шлётся дополнительно для надёжности
- Адрес: broadcast из конфига хоста
- Используется `net.DialUDP`, без внешних пакетов

**Парсинг MAC:** принимает форматы `AA:BB:CC:DD:EE:FF`, `aa-bb-cc-dd-ee-ff`, `AABBCCDDEEFF`.

### 5. Proxy (`internal/proxy`)

HTTP-клиент с опциональным SOCKS5-прокси и IPv6-first resolution.

**Режимы:**
1. **Без прокси** — обычный `http.Client` (с `prefer_ipv6` или без)
2. **SOCKS5 без авторизации** — `golang.org/x/net/proxy`
3. **SOCKS5 с авторизацией** — `proxy.Auth{User, Password}`

**IPv6-first dialer:** обёртка `proxy.ContextDialer`, которая резолвит AAAA-записи раньше A. Используется и как самостоятельный диалектор, и как `base` для SOCKS5.

**Таймауты:**
- `ResponseHeaderTimeout`: 75 с
- Общий `Timeout`: 90 с

### 6. Bot (`internal/bot`)

Обработка Telegram-сообщений и callback-запросов.

**Интерфейс — только inline-кнопки:**

| Кнопка | `callback_data` | Действие |
|--------|-----------------|----------|
| ✅ Включить | `wg_on` | `wg.Up()` + запуск таймера |
| ❌ Выключить | `wg_off` | Остановка таймера + `wg.Down()` |
| 🔄 Статус | `wg_status` | `wg.Show()` + форматирование |
| ⏱ Продлить | `scheduler_extend` | Перезапуск таймера |
| ⚡ WoL сервер | `wol_server` | Magic packet на первый хост |

**Команды (отладочные):**
- `/start` — главное меню
- `/status` — статус WG
- Текст «статус» — эквивалент `/status`

**Access control:**
- Проверка `IsAllowed()` на каждое сообщение и callback
- При отказе — `⛔ Доступ запрещён`

**Polling:** `GetUpdatesChan` с таймаутом 60 с, в цикле с контекстом.

### 7. Main (`cmd/wg-bot`)

Точка входа и graceful shutdown.

**Поток:**
1. `flag.Parse()` — путь к конфигу (по умолчанию `/opt/etc/wg-bot/config.toml`)
2. `config.Load()` — загрузка и валидация
3. `proxy.NewHTTPClient()` — создание HTTP-клиента
4. `tgbotapi.NewBotAPIWithClient()` — инициализация API
5. `bot.New()` — создание бота (создаёт WG-менеджер и таймер)
6. `signal.Notify(SIGINT, SIGTERM)` — ожидание сигнала
7. `b.Run(ctx)` — цикл polling'а
8. При получении сигнала — `cancel()`, корректное завершение

Логирование: `log.Ldate|log.Ltime|log.Lshortfile`.

## Безопасность

### Shell injection
Все вызовы `wg-quick` и `wg` — через `exec.CommandContext`, имя интерфейса подставляется как аргумент, shell не используется. Валидация: `#nosec G204` с комментарием о происхождении параметра из локального конфига.

### Access control
Бот отвечает только пользователям из белого списка `allowed_users`. Проверка — на каждое сообщение и callback. Пара `chat_id + user_id` должна совпадать полностью.

### Secrets
- `bot_token` только в `/opt/etc/wg-bot/config.toml` (`chmod 600`)
- `.gitignore` исключает `config.toml`
- В репозитории — только `config.toml.example` без реальных токенов

### CI security checks
- **gosec** — статический анализ на уязвимости
- **gitleaks** — поиск секретов в коммитах
- Оба запускаются в CI на каждый push и PR

## Сборка

### Локально

```bash
make all          # fmt → lint → security → test → build
make build        # кросс-компиляция: amd64 + mipsle softfloat
make release      # test + build + упаковка в tar.gz
make test         # go test -v -count=1 -race ./...
```

### Кросс-компиляция

| Цель | `GOOS` | `GOARCH` | `GOMIPS` | LDFLAGS |
|------|--------|----------|----------|---------|
| amd64 | linux | amd64 | — | `-s -w` |
| Keenetic | linux | mipsle | softfloat | `-s -w` |

Флаги `-s -w` убирают символы и DWARF-таблицу, уменьшая размер бинарника.

### GitHub Actions CI

```
push/PR → lint → gosec → gitleaks → test(-race) → build(amd64+mipsle)
         ↑_____ один джоб (Lint & Security) ______↑
                                                   ↓
                                         tag v* → release
```

Два джоба:
- **lint** — `golangci-lint` + `gosec` + `gitleaks`
- **test** — `go test -race` (параллельно с lint)

После них:
- **build** — кросс-компиляция, загрузка артефактов
- **release** (только для тегов `v*`) — упаковка `tar.gz` + GitHub Release

## Ключевые решения

| Решение | Значение | Причина |
|---------|----------|---------|
| Язык | Go | Статика, кросс-компиляция одной строкой, ноль зависимостей на роутере |
| Цель | MIPSEL soft-float | Entware на Keenetic — `mipsel-3.4`, аппаратное FPU отсутствует |
| GOARCH | `mipsle` + `GOMIPS=softfloat` | Эмуляция float в софте |
| Telegram API | `go-telegram-bot-api/v5` | Стандарт, pure Go |
| Конфиг | TOML | Человекочитаемый, с комментариями, строгая типизация |
| WoL | `net.DialUDP` → broadcast | Чистый сокет, без extern-пакетов |
| WG | `os/exec` → `wg-quick up/down` | `shell=false` — без инъекций |
| Таймер | `time.AfterFunc` + мьютекс | Живёт внутри процесса, `at`/`cron` не нужны |
| Прокси | `golang.org/x/net/proxy` | SOCKS5 из extended stdlib |
| Сборка | GitHub Actions | Lint + gosec + gitleaks + test + build + release |
| Интерфейс | Inline-клавиатура | Без текстовых команд, callback_data, мгновенная реакция |
