#!/bin/sh
# wg-bot — управление демоном wg-bot.
#
# Использование:
#   wg-bot start     — запустить
#   wg-bot stop      — остановить  
#   wg-bot restart   — перезапустить
#   wg-bot status    — проверить статус
#   wg-bot logs      — показать логи (tail -f)
#   wg-bot enable    — добавить в автозапуск
#   wg-bot disable   — убрать из автозапуска

BIN="/opt/sbin/wg-botd"
CONFIG="/opt/etc/wg-bot/config.toml"
PIDFILE="/opt/var/run/wg-bot.pid"
LOGFILE="/opt/var/log/wg-bot.log"

ensure_logdir() {
	mkdir -p "$(dirname "$LOGFILE")" 2>/dev/null
}

start() {
	if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
		echo "wg-bot already running (pid $(cat "$PIDFILE"))"
		return
	fi
	ensure_logdir
	echo "Starting wg-bot..."
	nohup "$BIN" -config "$CONFIG" >> "$LOGFILE" 2>&1 &
	echo $! > "$PIDFILE"
	sleep 1
	if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
		echo "Started (pid $(cat "$PIDFILE"))"
	else
		echo "Failed to start. Check logs: $LOGFILE"
		rm -f "$PIDFILE"
	fi
}

stop() {
	if [ ! -f "$PIDFILE" ]; then
		echo "wg-bot not running"
		return
	fi
	PID=$(cat "$PIDFILE")
	echo "Stopping wg-bot (pid $PID)..."
	kill "$PID" 2>/dev/null
	# Даём 10 сек на graceful shutdown
	for i in $(seq 1 10); do
		if ! kill -0 "$PID" 2>/dev/null; then
			break
		fi
		sleep 1
	done
	if kill -0 "$PID" 2>/dev/null; then
		echo "Force killing..."
		kill -9 "$PID" 2>/dev/null
	fi
	rm -f "$PIDFILE"
	echo "Stopped"
}

restart() {
	stop
	sleep 1
	start
}

status() {
	if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
		PID=$(cat "$PIDFILE")
		UPTIME=$(ps -o etime= -p "$PID" 2>/dev/null || echo "?")
		echo "wg-bot is running (pid $PID, uptime $UPTIME)"
		# Проверяем, отвечает ли бот (tail log)
		if [ -f "$LOGFILE" ]; then
			LAST=$(tail -1 "$LOGFILE" 2>/dev/null)
			[ -n "$LAST" ] && echo "Last log: $LAST"
		fi
	else
		echo "wg-bot is NOT running"
		return 1
	fi
}

logs() {
	if [ ! -f "$LOGFILE" ]; then
		echo "No logs yet"
		return
	fi
	tail -f "$LOGFILE"
}

enable_autostart() {
	if [ -f /opt/etc/init.d/S99wg-bot ]; then
		echo "Autostart already configured"
	else
		echo "Run install.sh to set up autostart"
	fi
}

disable_autostart() {
	if [ -f /opt/etc/init.d/S99wg-bot ]; then
		rm -f /opt/etc/init.d/S99wg-bot
		echo "Autostart disabled"
	else
		echo "Autostart not configured"
	fi
}

case "${1:-}" in
	start)   start ;;
	stop)    stop ;;
	restart) restart ;;
	status)  status ;;
	logs)    logs ;;
	enable)  enable_autostart ;;
	disable) disable_autostart ;;
	*)
		echo "Usage: $0 {start|stop|restart|status|logs|enable|disable}"
		exit 1
		;;
esac
