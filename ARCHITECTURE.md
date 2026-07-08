# Keenetic WireGuard Bot — Архитектура

## Проблема

На Keenetic с Entware настроен WireGuard-клиент (`wireguard-go`). Нужен Telegram-бот для:
- Вкл/выкл WG-туннеля
- Автоотключение через 30 мин после включения
- Отправка Wake-on-LAN на хост(ы)

## Окружение

- Платформа: Keenetic (MIPS/ARM, Entware, BusyBox ash)
- WG клиент: `wireguard-go` пакет, управляется через `wg-quick up/down wg0`
- Python: будет установлен через `opkg` + venv
- Процессы: нет systemd — Entware использует `/opt/etc/init.d/`

## Структура репозитория

```
keenetic-wg-bot/
├── README.md                    # Документация
├── ARCHITECTURE.md              # Этот файл
├── install.sh                   # Установщик (идемпотентный)
├── config.yaml.example          # Пример конфига (без секретов)
├── .gitignore
├── src/
│   ├── __init__.py
│   ├── main.py                  # Точка входа
│   ├── bot.py                   # Telegram-хендлеры
│   ├── wireguard.py             # Управление WG
│   ├── wol.py                   # Wake-on-LAN
│   ├── scheduler.py             # Планировщик автоотключения
│   └── config.py                # Загрузка конфига
└── scripts/
    └── wg-cron.sh               # cron-скрипт автоотключения (опционально)
```

## Компоненты

### 1. Config (`config.py`)

Конфиг в YAML: `/opt/etc/wg-bot/config.yaml` (chmod 600)

```yaml
telegram:
  bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
  allowed_users:
    - chat_id: 123456789
      user_id: 987654321

wireguard:
  interface: "wg0"
  config_path: "/opt/etc/wireguard/wg0.conf"

wol:
  hosts:
    - name: "asrock-server"
      mac: "AA:BB:CC:DD:EE:FF"
      broadcast: "192.168.1.255"

scheduler:
  auto_off_minutes: 30
  state_file: "/opt/etc/wg-bot/scheduler.state"
```

### 2. WireGuard control (`wireguard.py`)

Управление через subprocess:

| Действие | Команда |
|----------|---------|
| Включить | `wg-quick up wg0` |
| Выключить | `wg-quick down wg0` |
| Статус | `wg show wg0` |
| peer alive | Парсинг `wg show wg0` (latest handshake) |

Предварительно чиним `chmod 600 /opt/etc/wireguard/wg0.conf`.

### 3. WoL (`wol.py`)

`wakeonlan` из pip → `wakeonlan <MAC> -i <broadcast>`.
Либо raw socket: ethernet-фрейм с magic packet.

Храним: имя + MAC + broadcast IP в конфиге.

### 4. Scheduler (`scheduler.py`)

**Логика:**
1. Пользователь включил WG в боте
2. Записывается `scheduler.state` с меткой времени + указанием «выключить через 30 мин»
3. Ставится `at`-задача: `at now + 30 minutes` → `wg-quick down wg0`
4. Если пользователь выключает WG вручную раньше — `atrm` отменяет задачу
5. Если пользователь перезапускает таймер (например продлевает) — старая `at`-задача удаляется, ставится новая

**Почему `at`**, а не cron:
- `at` ставит одноразовую задачу на точное время
- Не нужно парсить cron и не нужно state polling
- `opkg install at` доступен в Entware

Резерв: если `at` недоступен — cron + state file с меткой времени (скрипт проверяет каждую минуту).

### 5. Telegram Bot (`bot.py`)

Библиотека: `pyTelegramBotAPI` **< 4.28** (без aiohttp, экономит сборку на MIPS).

**Команды:**
```
/start        — приветствие, проверка доступа
/status       — статус WG (вкл/выкл, handshake, трафик)
/on           — включить WG + запустить таймер 30 мин
/off          — выключить WG + отменить таймер
/timer N     — включить WG на N минут (переопределяет 30)
/wol <host>  — отправить WoL на хост (по имени из конфига)
/wol_list    — список хостов для WoL
```

**Клавиатура:** Reply/inline кнопки для /on, /off, /status.

### 6. Installer (`install.sh`)

Идемпотентный скрипт:

```bash
#!/bin/sh
# 1. Проверка: запущен ли на Keenetic с Entware
# 2. opkg update && opkg install python3 python3-pip python3-venv at wakeonlan
# 3. Копирование файлов из репо в /opt/wg-bot/
# 4. Создание venv: python3 -m venv /opt/wg-bot/venv
# 5. pip install -r /opt/wg-bot/requirements.txt
# 6. Создание /opt/etc/wg-bot/config.yaml (если нет — копия примера)
# 7. chmod 600 /opt/etc/wireguard/wg0.conf
# 8. Настройка автозапуска:
#    - Добавить �� /opt/etc/init.d/S99wg-bot (запуск от root)
#    (или через Entware rc.unslung)
# 9. Запуск бота
```

### 7. Init-скрипт (`/opt/etc/init.d/S99wg-bot`)

```sh
#!/bin/sh
# Standard Entware init script
START=99
start() {
    cd /opt/wg-bot && venv/bin/python src/main.py &
}
stop() {
    kill $(cat /opt/wg-bot/bot.pid 2>/dev/null)
}
```

### 8. Безопасность

- Конфиг с токеном — chmod 600
- `.gitignore` исключает `config.yaml` (только `.example`)
- Бот проверяет chat_id + user_id из белого списка
- `install.sh` не содержит секретов
- Все пароли/токены только в `/opt/etc/wg-bot/config.yaml`

## Поток: Включить WG на 30 мин

```
User → /on
  Bot: проверка auth
  Bot: запуск wg-quick up wg0
  Bot: проверка `wg show wg0` — alive
  Bot: запись scheduler.state (timestamp + on)
  Bot: at now + 30 min → wg-quick down wg0
  Bot: ✅ WG включён, отключится через 30 мин

  ... 30 мин проходит ...

  at → wg-quick down wg0
  Bot: (ничего — at запущен не в контексте бота)
  Но! scheduler.state проверяет: WG down → чистит state

  User → /status
  Bot: WG выключен (отключился по таймеру)
```

## Уточнения к тебе

Прежде чем писать код — пара финальных вопросов:

1. **WG интерфейс точно `wg0`?** Может быть `wg1` или другое имя.
2. **AsRock Z97 Extreme 4** — WoL в BIOS включён? Нужно будет слать broadcast на интерфейс, к которому подключён сервер (обычно LAN порт от Keenetic). Какой у сервера IP? (Для определения broadcast).
3. **Модель Keenetic** — MIPS или ARM? (От этого зависит архитектура пакетов в Entware, но `install.sh` сам определит).
4. **Доступ в интернет через WG?** После `wg-quick up wg0` — трафик идёт через туннель? Keenetic умеет селективно маршрутизировать, но надо понимать твою схему (весь трафик или только часть?).
5. **Приоритеты** — что важнее: простая установка или минимальный размер пакетов на MIPS?

Утверждаешь архитектуру? Если всё ок — пишу код.
