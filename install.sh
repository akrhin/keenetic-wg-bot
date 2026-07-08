#!/bin/sh
# install.sh — установка wg-bot на Keenetic с Entware.
# Идемпотентно: можно запускать повторно для обновления.
#
# Использование:
#   curl -sSL https://raw.githubusercontent.com/akrhin/keenetic-wg-bot/main/install.sh | bash
#   # или из локального архива:
#   ./install.sh
set -e

# ── Прокси ─────────────────────────────────────────────────────
# Раскомментируй и укажи свой прокси для скачивания бинарника и скриптов.
# Пример с авторизацией: socks5h://user:pass@192.168.1.139:1080
# export ALL_PROXY="socks5h://192.168.1.139:1080"
# export HTTP_PROXY="socks5h://192.168.1.139:1080"
# export HTTPS_PROXY="socks5h://192.168.1.139:1080"
# export NO_PROXY="127.0.0.1,localhost"

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

# ── Универсальная загрузка (curl → wget) ──────────────────────
detect_downloader() {
	if command -v curl >/dev/null 2>&1; then
		echo "curl"
	elif command -v wget >/dev/null 2>&1; then
		echo "wget"
	else
		echo ""
	fi
}

# $1 = URL, $2 = output file
download_file() {
	local url="$1"
	local out="$2"
	local dl
	dl=$(detect_downloader)

	case "$dl" in
		curl)
			curl -sSL --connect-timeout 15 --max-time 60 -o "$out" "$url"
			;;
		wget)
			wget -q --timeout=60 -O "$out" "$url"
			;;
		*)
			error "Neither curl nor wget found."
			return 1
			;;
	esac
	return $?
}

verify_gzip() {
	# Проверяем, что файл — настоящий gzip: tar -tzf читает оглавление.
	# Это работает на любом BusyBox, без заморочек с od.
	tar -tzf "$1" >/dev/null 2>&1
}

# ── Функция обновления бинарника ──────────────────────────────
update_binary() {
	local LATEST_URL
	LATEST_URL="https://github.com/${REPO}/releases/latest/download/wg-bot-mipsle.tar.gz"
	local WORKDIR
	WORKDIR=$(mktemp -d)
	local TMPTGZ="${WORKDIR}/archive.tar.gz"

	# Гарантированно глушим процесс перед обновлением
	killall wg-botd 2>/dev/null || true
	sleep 1

	info "Downloading $LATEST_URL ..."
	download_file "$LATEST_URL" "$TMPTGZ" || {
		error "Failed to download binary."
		rm -rf "$WORKDIR"
		return 1
	}

	if ! verify_gzip "$TMPTGZ"; then
		error "Downloaded file is not a gzip archive. Size: $(wc -c < "$TMPTGZ") bytes."
		error "File starts with (hex): $(head -c 8 "$TMPTGZ" | od -tx1 2>/dev/null | head -1)"
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

DL=$(detect_downloader)
if [ -z "$DL" ]; then
	error "Neither curl nor wget available. Install one of them first."
	exit 1
fi
info "Using $DL for downloads"

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
else
	info "Downloading wg-bot binary from GitHub..."
	LATEST_URL="https://github.com/${REPO}/releases/latest/download/wg-bot-mipsle.tar.gz"
	WORKDIR=$(mktemp -d)
	TMPTGZ="${WORKDIR}/archive.tar.gz"

	download_file "$LATEST_URL" "$TMPTGZ" || {
		error "Failed to download binary."
		rm -rf "$WORKDIR"
		exit 1
	}

	if ! verify_gzip "$TMPTGZ"; then
		error "Downloaded file is not a gzip archive. Size: $(wc -c < "$TMPTGZ") bytes."
		error "File starts with (hex): $(head -c 8 "$TMPTGZ" | od -tx1 2>/dev/null | head -1)"
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
fi

# ── Скрипт управления ─────────────────────────────────────────
info "Installing management script..."
download_file \
	"https://raw.githubusercontent.com/${REPO}/main/scripts/wg-bot.sh" \
	"$CTL" && chmod 755 "$CTL" && info "Management script: $CTL" \
	|| warn "Failed to download management script"

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
	download_file \
		"https://raw.githubusercontent.com/${REPO}/main/config.toml.example" \
		"${CONFIG_DIR}/config.toml" \
		&& chmod 600 "${CONFIG_DIR}/config.toml" \
		&& info "Created ${CONFIG_DIR}/config.toml from repository example" \
		|| warn "Failed to download example config"
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
