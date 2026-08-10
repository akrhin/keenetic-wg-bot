# keenetic-wg-bot

Telegram-бот для управления WireGuard и Wake-on-LAN на роутерах Keenetic с Entware.

## Возможности

- ✅ **Вкл/Выкл WireGuard** через inline-кнопки (wg-quick up/down)
- ⏱ **Автоотключение** — таймер на N минут (по умолчанию 30)
- 🔄 **Статус WG** — жив ли туннель, порт, количество пиров
- ⚡ **Wake-on-LAN** — кнопка «разбудить сервер»
- 🔐 **Доступ** — только по белому списку chat_id + user_id
- 🌐 **SOCKS5-прокси** для Telegram API (с авторизацией или без)
- 🌍 **IPv6-first** — резолв AAAA раньше A
- 🔁 **Self-healing** — при глюке сети автоматически пересоздаёт соединение, без ручного рестарта
- 🛡 **Rate-limit** — cooldown команд + автобан за перебор доступа

## Установка

```bash
# Прямая установка на Keenetic (из SSH)
curl -sSL https://raw.githubusercontent.com/akrhin/keenetic-wg-bot/main/install.sh | bash

# После установки — отредактировать конфиг
nano /opt/etc/wg-bot/config.toml

# Управление
wg-bot start      # запустить
wg-bot stop       # остановить
wg-bot restart    # перезапустить
wg-bot status     # проверить статус
wg-bot logs       # смотреть логи
wg-bot enable     # добавить в автозапуск
wg-bot disable    # убрать из автозапуска
```

## Конфигурация

`/opt/etc/wg-bot/config.toml`:

```toml
[telegram]
bot_token = "YOUR_BOT_TOKEN"

[[telegram.allowed_users]]
chat_id = 123456789
user_id = 987654321

[wireguard]
interface = "wg0"
config_path = "/opt/etc/wireguard/wg0.conf"

[[wol.hosts]]
name = "asrock-server"
mac = "00:11:22:33:44:55"
broadcast = "192.168.1.255"

[scheduler]
auto_off_minutes = 30

[network]
# prefer_ipv6 = true   # резолвить AAAA (IPv6) раньше A (IPv4)
# poll_timeout = 30    # long-polling (сек); ниже порога обрыва idle-соединений сети

[security]
# command_cooldown_seconds = 2   # мин. интервал между командами одного чата
# max_failed_attempts = 5        # неудачных попыток доступа до бана чата
# ban_seconds = 600              # длительность бана после перебора доступа

[proxy]
# enabled = true
# url = "socks5://user:pass@192.168.1.100:1080"
```

## Интерфейс

Бот использует только inline-кнопки — никаких текстовых команд.

| Кнопка | Действие |
|--------|----------|
| ✅ Включить | `wg-quick up` + запуск таймера |
| ❌ Выключить | Остановка таймера + `wg-quick down` |
| 🔄 Статус | `wg show dump` + форматирование |
| ⏱ Продлить | Перезапуск таймера на N минут |
| ⚡ WoL сервер | Magic packet на broadcast UDP 9 + 7 |

**Отладочные команды:** `/start` (главное меню), `/status` или просто «статус».

## Прокси

Поддерживается SOCKS5 с авторизацией и без. Включение — через `proxy.enabled = true`. 
Настройка доступна при развёртывании (см. `ALL_PROXY` в install.sh) и в рантайме.

## Надёжность

Бот устойчив к сбоям сети и не требует ручного рестарта при «зависшем» соединении:

- **Собственный polling-цикл** вместо `GetUpdatesChan` — с контролем ошибок, backoff'ом и защитой от паник в обработчиках;
- **Пересоздание транспорта** — после N ошибок подряд HTTP-клиент пересобирается, «протухшие» keep-alive соединения закрываются;
- **Таймауты в сети** — DNS (5с), соединение (15с), idle-соединения (90с) — даже мёртвый локальный DNS не вешает бота;
- **Ретраи на старте** — при временном глюке DNS/прокси бот не падает, а пробует подключиться снова;
- **Восстановление таймера** — если после перезагрузки WG остался поднят, авто-выключение перезапускается автоматически.

## Безопасность

- **Shell injection** — все вызовы `wg-quick`/`wg` через `exec.Command` без shell
- **Access control** — белый список пар `chat_id + user_id`
- **Secrets** — `bot_token` только в `/opt/etc/wg-bot/config.toml` (chmod 600)
- **Логи без токенов** — токен маскируется (`***`) во всех ошибках и логах, включая ретраи Telegram API
- **Rate-limit** — cooldown между командами; автобан чата после перебора доступа (секция `[security]`)
- **Целостность бинарника** — `install.sh` проверяет SHA-256 скачанного архива
- **CI security checks** — gosec + gitleaks + govulncheck на каждый push

## Сборка из исходников

Требуется Go 1.26+ (версия зафиксирована в `go.mod`, тулчейн подтягивается автоматически).

```bash
# Для Keenetic (MIPSEL soft-float)
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o wg-bot ./cmd/wg-bot/

# Для теста на x86_64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o wg-bot ./cmd/wg-bot/

# Полный цикл
make all          # fmt → lint → security → test → build
make release      # test + build + tar.gz
```

## CI/CD

GitHub Actions при пуше:

1. `golangci-lint` — проверка стиля
2. `gosec` — статический анализ уязвимостей
3. `gitleaks` — поиск секретов в коде
4. `go vet` — статический анализ Go
5. `govulncheck` — проверка зависимостей на CVE
6. `go test -race` — тесты без гонок (51 тест, 6 пакетов)
7. Сборка под amd64 + mipsle (softfloat)

При создании тега `v*` — автоматический релиз на GitHub.

## Архитектура

```
cmd/wg-bot/main.go          — точка входа + graceful shutdown
internal/
  bot/bot.go                — Telegram-хендлеры + inline-кнопки
  config/config.go          — загрузка + валидация TOML
  proxy/proxy.go            — SOCKS5 + IPv6-first dialer
  wireguard/wg.go           — управление wg-quick (exec.Command, без shell)
  wol/wol.go                — отправка WoL magic packet (UDP 9 + 7)
  scheduler/timer.go        — таймер автоотключения (time.AfterFunc + мьютекс)
install.sh                  — идемпотентный установщик на Entware
scripts/wg-bot.sh           — скрипт управления (start/stop/restart/status/logs)
```

Детальная архитектура: [`ARCHITECTURE.md`](ARCHITECTURE.md)
Контекст для LLM: [`AGENTS.md`](AGENTS.md)

## Лицензия и отказ от ответственности

MIT License

Copyright (c) 2026 akrhin

Данное программное обеспечение разработано с использованием генеративных языковых моделей (ИИ).  
Автор не несёт ответственности за любые последствия, прямые или косвенные, связанные с использованием данного программного обеспечения, включая, но не ограничиваясь: потерей данных, нарушением работы оборудования, финансовыми потерями или иными убытками.

Программное обеспечение предоставляется «как есть», без каких-либо гарантий, явных или подразумеваемых.

Полный текст: [`LICENSE`](LICENSE)
