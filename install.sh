#!/bin/sh
# install.sh — установка wg-bot на Keenetic с Entware.
# Идемпотентно: можно запускать повторно для обновления.
#
# Использование:
#   curl -sSL https://raw.githubusercontent.com/akrhin/keenetic-wg-bot/main/install.sh | bash
#   # или из локального архива:
#   ./install.sh
set -e

# ── Цвета ─────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { printf "${GREEN}[✓]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[!]${NC} %s\n" "$1"; }
error() { printf "${RED}[✗]${NC} %s\n" "$1"; }

REPO="akrhin/keenetic-wg-bot"
BIN="/opt/bin/wg-bot"
CONFIG_DIR="/opt/etc/wg-bot"
INIT_SCRIPT="/opt/etc/init.d/S99wg-bot"
PIDFILE="/opt/var/run/wg-bot.pid"
CONFIG="${CONFIG_DIR}/config.toml"

# ── Проверка окружения ────────────────────────────────────────
if [ ! -d /opt/bin ]; then
	error "Entware not found (/opt/bin missing). Install Entware first."
	exit 1
fi

# ── Установка пакетов ─────────────────────────────────────────
info "Updating package lists..."
opkg update 2>/dev/null || warn "opkg update failed, continuing..."

for pkg in wireguard-tools wireguard-go; do
	if ! opkg list-installed 2>/dev/null | grep -q "^${pkg} "; then
		info "Installing $pkg..."
		opkg install "$pkg" || warn "Failed to install $pkg"
	else
		info "$pkg already installed"
	fi
done

# ── Скачивание/копирование бинарника ──────────────────────────
if [ -f "$BIN" ]; then
	info "Binary already installed: $BIN"
elif [ -f "$(dirname "$0")/wg-bot" ]; then
	# Установка из локального архива
	info "Installing binary from local file..."
	cp "$(dirname "$0")/wg-bot" "$BIN"
	chmod 755 "$BIN"
elif command -v wget >/dev/null 2>&1; then
	info "Downloading wg-bot binary from GitHub..."
	LATEST_URL="https://github.com/${REPO}/releases/latest/download/wg-bot-mipsle.tar.gz"
	WORKDIR=$(mktemp -d)
	wget -qO- "$LATEST_URL" | tar xz -C "$WORKDIR" || {
		error "Failed to download binary. Check connection or install manually."
		rm -rf "$WORKDIR"
		exit 1
	}
	mv "$WORKDIR/wg-bot" "$BIN"
	chmod 755 "$BIN"
	rm -rf "$WORKDIR"
	info "Binary downloaded: $BIN"
else
	error "No wg-bot binary found and wget not available."
	error "Download from: https://github.com/${REPO}/releases/latest"
	exit 1
fi

# ── Фикс прав на WG-конфиг ────────────────────────────────────
if [ -f /opt/etc/wireguard/wg0.conf ]; then
	chmod 600 /opt/etc/wireguard/wg0.conf
	info "Fixed permissions on wg0.conf"
fi

# ── Конфиг ─────────────────────────────────────────────────────
if [ ! -d "$CONFIG_DIR" ]; then
	mkdir -p "$CONFIG_DIR"
	chmod 700 "$CONFIG_DIR"
	info "Created $CONFIG_DIR"
fi

if [ ! -f "$CONFIG" ]; then
	# Скачиваем пример конфига из репозитория
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$CONFIG" \
			"https://raw.githubusercontent.com/${REPO}/main/config.toml.example" \
			&& chmod 600 "$CONFIG" \
			&& info "Created $CONFIG from repository example" \
			|| warn "Failed to download example config"
	else
		warn "Create ${CONFIG} manually, see:"
		warn "  https://github.com/${REPO}/blob/main/config.toml.example"
	fi
else
	info "Config exists: $CONFIG"
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
echo "  1. Edit ${CONFIG} with your Telegram token and IDs"
echo "  2. /opt/etc/init.d/S99wg-bot start"
echo ""

# Запуск, если не в curl-пайпе
if [ -t 0 ]; then
	if [ -f "$BIN" ]; then
		$BIN -config "$CONFIG" &
		echo "  Bot started in background. Check logs."
	fi
fi
