# keenetic-wg-bot

Telegram-бот для управления WireGuard и Wake-on-LAN на Keenetic с Entware.

## Возможности

- ✅ **Вкл/Выкл WireGuard** через inline-кнопки
- ⏱ **Автоотключение** через 30 минут (настраивается)
- 🔄 **Статус WG** — жив ли туннель, сколько пиров
- ⚡ **Wake-on-LAN** — кнопка «разбудить сервер»
- 🔐 **Доступ** — только по белому списку chat_id + user_id

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
bot_token = "1234567890:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

[[telegram.allowed_users]]
chat_id = 123456789
user_id = 987654321

[wireguard]
interface = "wg0"

[[wol.hosts]]
name = "asrock-server"
mac = "00:11:22:33:44:55"
broadcast = "192.168.1.255"

[scheduler]
auto_off_minutes = 30
```

## Сборка из исходников

```bash
# Для Keenetic (MIPSEL soft-float)
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o wg-bot ./cmd/wg-bot/

# Для теста на x86_64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o wg-bot ./cmd/wg-bot/
```

## CI/CD

GitHub Actions при пуше:
1. `golangci-lint` — проверка стиля
2. `gosec` — статический анализ уязвимостей
3. `gitleaks` — проверка секретов
4. `go test -race` — тесты
5. Сборка под amd64 + mipsle

При создании тега `v*` — автоматический релиз.

## Архитектура

```
cmd/wg-bot/main.go          — точка входа + graceful shutdown
internal/
  bot/bot.go                — Telegram-хендлеры + inline-кнопки
  config/config.go          — загрузка TOML-конфига
  wireguard/wg.go           — управление wg-quick
  wol/wol.go                — отправка WoL magic packet
  scheduler/timer.go        — таймер автоотключения
install.sh                  — идемпотентный установщик
```

## Лицензия

MIT
