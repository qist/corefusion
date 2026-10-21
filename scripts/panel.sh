#!/usr/bin/env bash
# CoreFusion 面板启停脚本
#
# 用法:
#   scripts/panel.sh start|stop|restart|status|logs
#
# 可通过环境变量覆盖:
#   BIN           面板二进制路径（默认 <repo>/corefusion）
#   PIDFILE       PID 文件（默认 <repo>/data/corefusion.pid）
#   LOGFILE       日志文件（默认 <repo>/data/corefusion.log）
#   START_TIMEOUT 启动后确认进程存活的等待秒数（默认 3）
#   STOP_TIMEOUT  停止时等待进程退出的秒数（默认 30）
#
# 注意：数据库默认使用相对路径（sqlite://data/sqlite/app.db），
#       脚本会先切换到仓库根目录再启动。

set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${BIN:-$BASE_DIR/corefusion}"
PIDFILE="${PIDFILE:-$BASE_DIR/data/corefusion.pid}"
LOGFILE="${LOGFILE:-$BASE_DIR/data/corefusion.log}"
START_TIMEOUT="${START_TIMEOUT:-3}"
STOP_TIMEOUT="${STOP_TIMEOUT:-30}"

BIN_NAME="$(basename "$BIN")"

read_pid() {
	if [[ -s "$PIDFILE" ]]; then
		cat "$PIDFILE"
	fi
}

pid_alive() {
	local pid="$1"
	[[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

# 查找同名进程（用于识别不是本脚本启动的实例）
find_process() {
	pgrep -x "$BIN_NAME" 2>/dev/null | head -n 1 || true
}

require_binary() {
	if [[ ! -x "$BIN" ]]; then
		echo "错误：找不到可执行文件 $BIN，请先执行 make panel" >&2
		exit 1
	fi
}

start_panel() {
	local pid
	pid="$(read_pid)"
	if pid_alive "$pid"; then
		echo "面板已在运行（PID $pid）"
		return 0
	fi

	local other
	other="$(find_process)"
	if [[ -n "$other" ]]; then
		echo "错误：检测到已在运行的 $BIN_NAME 进程（PID $other），但 PID 文件不匹配。" >&2
		echo "      请先停止旧进程，或手动将其 PID 写入 $PIDFILE 后再使用本脚本。" >&2
		exit 1
	fi

	require_binary

	mkdir -p "$(dirname "$PIDFILE")" "$(dirname "$LOGFILE")"
	cd "$BASE_DIR"

	setsid nohup "$BIN" server >>"$LOGFILE" 2>&1 </dev/null &
	pid=$!
	echo "$pid" >"$PIDFILE"

	sleep "$START_TIMEOUT"
	if ! pid_alive "$pid"; then
		echo "错误：面板启动失败，最近日志：" >&2
		tail -n 30 "$LOGFILE" >&2 || true
		rm -f "$PIDFILE"
		exit 1
	fi

	echo "面板已启动（PID $pid）"
	echo "  日志：$LOGFILE"
}

stop_panel() {
	local pid
	pid="$(read_pid)"

	if ! pid_alive "$pid"; then
		rm -f "$PIDFILE"
		local other
		other="$(find_process)"
		if [[ -n "$other" ]]; then
			echo "未找到 PID 文件对应的进程，但检测到 $BIN_NAME 进程（PID $other）。" >&2
			echo "该进程不是本脚本启动的，请确认后手动停止：kill $other" >&2
			exit 1
		fi
		echo "面板未运行"
		return 0
	fi

	echo "正在停止面板（PID $pid）..."
	kill -TERM "$pid" 2>/dev/null || true

	local i
	for ((i = 0; i < STOP_TIMEOUT; i++)); do
		if ! pid_alive "$pid"; then
			rm -f "$PIDFILE"
			echo "面板已停止"
			return 0
		fi
		sleep 1
	done

	echo "等待 ${STOP_TIMEOUT}s 后仍未退出，强制结束（PID $pid）"
	kill -KILL "$pid" 2>/dev/null || true
	rm -f "$PIDFILE"
	echo "面板已强制停止"
}

status_panel() {
	local pid
	pid="$(read_pid)"

	if pid_alive "$pid"; then
		echo "面板运行中（PID $pid）"
		echo "  二进制：$BIN"
		echo "  日志：  $LOGFILE"
		return 0
	fi

	local other
	other="$(find_process)"
	if [[ -n "$other" ]]; then
		echo "面板运行中（PID $other，非本脚本启动，无 PID 文件记录）"
		return 0
	fi

	echo "面板未运行"
	return 3
}

logs_panel() {
	if [[ ! -f "$LOGFILE" ]]; then
		echo "日志文件不存在：$LOGFILE" >&2
		exit 1
	fi
	tail -n 200 -f "$LOGFILE"
}

case "${1:-}" in
start) start_panel ;;
stop) stop_panel ;;
restart)
	stop_panel
	start_panel
	;;
status) status_panel ;;
logs) logs_panel ;;
*)
	echo "用法: $0 {start|stop|restart|status|logs}" >&2
	exit 2
	;;
esac
