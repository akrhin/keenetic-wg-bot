#!/bin/sh
# install.sh — установка wg-bot на Keenetic с Entware.
#
# Идемпотентно: можно запускать повторно для обновления.
set -e

# ── Цвета ─────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { printf "${GREEN}[✓]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[!]${NC} %s\n" "$1"; }
error() { printf "${RED}[✗]${NC} %s\n" "$1"; }

# ── Проверка окружения ────────────────────────────────────────
if [ ! -d /opt/bin ]; then
	error "Entware not found (/opt/bin missing). Install Entware first."
	exit 1
fi

if [ ! -f /opt/bin/opkg ]; then
	error "opkg not found. Is Entware installed correctly?"
	exit 1
fi

BIN="/opt/bin/wg-bot"
CONFIG_DIR="/opt/etc/wg-bot"
INIT_SCRIPT="/opt/etc/init.d/S99wg-bot"
PIDFILE="/opt/var/run/wg-bot.pid"

# ── Установка пакетов ─────────────────────────────────────────
info "Updating package lists..."
opkg update 2>/dev/null || warn "opkg update failed, continuing..."

for pkg in python3 wireguard-tools wireguard-go; do
	if ! opkg list-installed 2>/dev/null | grep -q "^${pkg} "; then
		info "Installing $pkg..."
		opkg install "$pkg" || warn "Failed to install $pkg"
	else
		info "$pkg already installed"
	fi
done

# ── Копирование бинарника ─────────────────────────────────────
THIS_SCRIPT="$(readlink -f "$0")"
SCRIPT_DIR="$(dirname "$THIS_SCRIPT")"

if [ -f "${SCRIPT_DIR}/wg-bot" ]; then
	info "Installing binary..."
	cp "${SCRIPT_DIR}/wg-bot" "$BIN"
	chmod 755 "$BIN"
	chown root:root "$BIN"
else
	warn "Binary not found next to install.sh. Place wg-bot in the same directory."
	warn "Expected: ${SCRIPT_DIR}/wg-bot"
fi

# ── Конфиг ─────────────────���──────────────────────────────────
if [ ! -d "$CONFIG_DIR" ]; then
	mkdir -p "$CONFIG_DIR"
	chmod 700 "$CONFIG_DIR"
	info "Created $CONFIG_DIR"
fi

if [ ! -f "${CONFIG_DIR}/config.toml" ]; then
	if [ -f "${SCRIPT_DIR}/config.toml.example" ]; then
		cp "${SCRIPT_DIR}/config.toml.example" "${CONFIG_DIR}/config.toml"
		chmod 600 "${CONFIG_DIR}/config.toml"
		info "Created ${CONFIG_DIR}/config.toml from example"
		warn "!!! EDIT ${CONFIG_DIR}/config.toml WITH YOUR TOKEN AND IDs !!!"
	else
		warn "config.toml.example not found. Create ${CONFIG_DIR}/config.toml manually."
	fi
else
	info "Config exists: ${CONFIG_DIR}/config.toml"
fi

# ── Фикс прав на WG-конфиг ────────────────────────────────────
if [ -f /opt/etc/wireguard/wg0.conf ]; then
	chmod 600 /opt/etc/wireguard/wg0.conf
	info "Fixed permissions on wg0.conf"
fi

# ── Init-скрипт ───────────────────────────────────────────────
if [ ! -f "$INIT_SCRIPT" ]; then
	cat > "$INIT_SCRIPT" <<'INITEOF'
#!/bin/sh
# wg-bot init script for Entware
START=99
PIDFILE=/opt/var/run/wg-bot.pid
BIN=/opt/bin/wg-bot
CONFIG=/opt/etc/wg-bot/config.toml

start() {
	if [ -f "$PIDFILE" ]; then
		kill -0 "$(cat "$PIDFILE")" 2>/dev/null && return
	fi
	echo "Starting wg-bot..."
	$BIN -config "$CONFIG" &
	echo $! > "$PIDFILE"
}

stop() {
	if [ -f "$PIDFILE" ]; then
		kill "$(cat "$PIDFILE")" 2>/dev/null && rm -f "$PIDFILE" && echo "Stopped wg-bot"
	fi
}

restart() {
	stop
	sleep 1
	start
}

case "$1" in
	start)   start ;;
	stop)    stop ;;
	restart) restart ;;
	*)       echo "Usage: $0 {start|stop|restart}" ;;
esac
INITEOF
	chmod 755 "$INIT_SCRIPT"
	info "Created init script: $INIT_SCRIPT"
else
	info "Init script exists: $INIT_SCRIPT"
fi

# ── Финиш ─────────────────────────────────────────────────────
echo ""
info "Installation complete!"
echo ""
echo "  Next steps:"
echo "  1. Edit ${CONFIG_DIR}/config.toml with your settings"
echo "  2. /opt/etc/init.d/S99wg-bot start"
echo "  3. /opt/etc/init.d/S99wg-bot stop"
echo ""

if [ -f "$BIN" ]; then
	"$BIN" -config "${CONFIG_DIR}/config.toml" &
	echo "  Bot started in background. Check logs."
fi
