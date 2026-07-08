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
BIN="/opt/sbin/wg-botd"
CTL="/opt/bin/wg-bot"
CONFIG_DIR="/opt/etc/wg-bot"
INIT_SCRIPT="/opt/etc/init.d/S99wg-bot"

# ── Функция обновления бинарника ──────────────────────────────
update_binary() {
	local LATEST_URL
	local LATEST_URL="https://github.com/${REPO}/releases/latest/download/wg-bot-mipsle.tar.gz"
	local WORKDIR
	WORKDIR=$(mktemp -d)
	local TMPTGZ="${WORKDIR}/archive.tar.gz"

	# Гарантированно глушим процесс перед обновлением
	killall wg-botd 2>/dev/null || true
	sleep 1

	# Скачиваем во временный файл, проверяем magic
	wget -q --timeout=30 -O "$TMPTGZ" "$LATEST_URL" || {
		error "Failed to download binary (wget exit code $?)."
		rm -rf "$WORKDIR"
		return 1
	}
	# Проверка: gzip-архив начинается с \x1f\x8b
	if [ "$(head -c 2 "$TMPTGZ" | od -An -tx1 | tr -d ' ')" != "1f8b" ]; then
		error "Downloaded file is not a gzip archive. Size: $(wc -c < "$TMPTGZ") bytes."
		error "First bytes: $(head -c 200 "$TMPTGZ" | tr '\0-\177' '.' | head -c 200)"
		rm -rf "$WORKDIR"
		return 1
	fi

	tar xzf "$TMPTGZ" -C "$WORKDIR" || {
		error "Failed to extract archive."
		rm -rf "$WORKDIR"
		return 1
	}

	if [ -f "$WORKDIR/wg-bot" ]; then
		# Двойной kill на случай если init-скрипт не успел
		killall wg-botd 2>/dev/null || true
		sleep 1
		cp "$WORKDIR/wg-bot" "$BIN"
		chmod 755 "$BIN"
		rm -rf "$WORKDIR"
		info "Binary updated: $BIN"
		# Запускаем заново
		"$CTL" start 2>/dev/null || "$INIT_SCRIPT" start 2>/dev/null || true
		return 0
	fi

	error "Unknown binary name in archive. Files: $(ls "$WORKDIR")"
	rm -rf "$WORKDIR"
	return 1
}

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
	info "Updating binary: $BIN"
	update_binary
elif [ -f "$(dirname "$0")/wg-bot" ]; then
	info "Installing binary from local file..."
	cp "$(dirname "$0")/wg-bot" "$BIN"
	chmod 755 "$BIN"
elif command -v wget >/dev/null 2>&1; then
	info "Downloading wg-bot binary from GitHub..."
	LATEST_URL="https://github.com/${REPO}/releases/latest/download/wg-bot-mipsle.tar.gz"
	WORKDIR=$(mktemp -d)
	TMPTGZ="${WORKDIR}/archive.tar.gz"
	wget -q --timeout=30 -O "$TMPTGZ" "$LATEST_URL" || {
		error "Failed to download binary."
		rm -rf "$WORKDIR"
		exit 1
	}
	if [ "$(head -c 2 "$TMPTGZ" | od -An -tx1 | tr -d ' ')" != "1f8b" ]; then
		error "Downloaded file is not a gzip archive. Size: $(wc -c < "$TMPTGZ") bytes."
		error "URL may be blocked or returning error page. Try: wget -q -O /tmp/test.tar.gz \"$LATEST_URL\" && file /tmp/test.tar.gz"
		rm -rf "$WORKDIR"
		exit 1
	fi
	tar xzf "$TMPTGZ" -C "$WORKDIR" || {
		error "Failed to extract archive."
		rm -rf "$WORKDIR"
		exit 1
	}
	# бинарник в архиве называется wg-bot (v0.2+) либо wg-bot-mipsle (v0.1)
	if [ -f "$WORKDIR/wg-bot" ]; then
		mv "$WORKDIR/wg-bot" "$BIN"
	elif [ -f "$WORKDIR/wg-bot-mipsle" ]; then
		mv "$WORKDIR/wg-bot-mipsle" "$BIN"
	else
		error "Unknown binary name in archive. Files: $(ls "$WORKDIR")"
		rm -rf "$WORKDIR"
		exit 1
	fi
	chmod 755 "$BIN"
	rm -rf "$WORKDIR"
	info "Binary downloaded: $BIN"
else
	error "No wg-bot binary found and wget not available."
	error "Download from: https://github.com/${REPO}/releases/latest"
	exit 1
fi

# ── Скрипт управления ─────────────────────────────────────────
info "Installing management script..."
if command -v wget >/dev/null 2>&1; then
	wget -qO "$CTL" \
		"https://raw.githubusercontent.com/${REPO}/main/scripts/wg-bot.sh" \
		&& chmod 755 "$CTL" \
		&& info "Management script: $CTL" \
		|| warn "Failed to download management script"
else
	warn "Place scripts/wg-bot.sh as $CTL manually"
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

if [ ! -f "${CONFIG_DIR}/config.toml" ]; then
	if command -v wget >/dev/null 2>&1; then
		wget -qO "${CONFIG_DIR}/config.toml" \
			"https://raw.githubusercontent.com/${REPO}/main/config.toml.example" \
			&& chmod 600 "${CONFIG_DIR}/config.toml" \
			&& info "Created ${CONFIG_DIR}/config.toml from repository example" \
			|| warn "Failed to download example config"
	else
		warn "Create ${CONFIG_DIR}/config.toml manually, see:"
		warn "  https://github.com/${REPO}/blob/main/config.toml.example"
	fi
else
	info "Config exists: ${CONFIG_DIR}/config.toml"
fi

# ── Init-скрипт ───────────────────────────────────────────────
if [ ! -f "$INIT_SCRIPT" ]; then
	cat > "$INIT_SCRIPT" <<'INITEOF'
#!/bin/sh
# wg-bot init script for Entware
# Делегирует всё скрипту управления /opt/bin/wg-bot
START=99

CMD="/opt/bin/wg-bot"

case "$1" in
	start)   $CMD start ;;
	stop)    $CMD stop ;;
	restart) $CMD restart ;;
	status)  $CMD status ;;
	*)       echo "Usage: $0 {start|stop|restart|status}" ;;
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
echo "  Available commands:"
echo "    wg-bot status   — проверить статус"
echo "    wg-bot start    — запустить"
echo "    wg-bot stop     — остановить"
echo "    wg-bot restart  — перезапустить"
echo "    wg-bot logs     — смотреть логи"
echo "    wg-bot enable   — включить автозапуск"
echo "    wg-bot disable  — отключить автозапуск"
echo ""
echo "  Config: ${CONFIG_DIR}/config.toml"
echo "  Binary: $BIN"
echo "  Logs:   /opt/var/log/wg-bot.log"
echo ""
warn "Edit ${CONFIG_DIR}/config.toml with your Telegram token and IDs, then:"
echo "    wg-bot start"
