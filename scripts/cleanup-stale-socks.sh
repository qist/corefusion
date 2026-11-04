#!/usr/bin/env bash
# 清理「没有进程监听」的 unix socket 文件。
#
# 场景：xray / sing-box 被 SIGKILL、OOM 或崩溃时，*net.UnixListener 来不及 unlink
# socket 文件；下次启动 bind 同一路径会 EADDRINUSE，表现为核心永远起不来。
#
# 安全判定（两者都满足才删）：
#   1. `ss -xl` 里没有该路径（没有进程在 listen）
#   2. 能非阻塞拿到 <path>.lock 的 flock（xray 用 flock 做单实例互斥）
#
# 用法：
#   sudo bash scripts/cleanup-stale-socks.sh            # 清理 /dev/shm/*.sock
#   sudo bash scripts/cleanup-stale-socks.sh /run/xray  # 指定目录
#   sudo bash scripts/cleanup-stale-socks.sh --dry-run  # 只打印
#
# systemd 过渡方案（xray-ui.service / corefusion.service）：
#   [Service]
#   ExecStartPre=-/usr/local/bin/cleanup-stale-socks.sh

set -u

DRY_RUN=0
DIRS=()
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    *) DIRS+=("$arg") ;;
  esac
done
if [ ${#DIRS[@]} -eq 0 ]; then
  DIRS=("/dev/shm")
fi

removed=0
for dir in "${DIRS[@]}"; do
  [ -d "$dir" ] || continue
  for sock in "$dir"/*.sock; do
    [ -S "$sock" ] || continue

    # 1) 有进程在 listen 就不动
    if command -v ss >/dev/null 2>&1 && ss -xl 2>/dev/null | grep -qF -- "$sock"; then
      continue
    fi

    # 2) xray 的 .lock 被持有说明还有存活实例
    if [ -e "$sock.lock" ] && command -v flock >/dev/null 2>&1; then
      if ! flock -n "$sock.lock" true 2>/dev/null; then
        continue
      fi
    fi

    if [ "$DRY_RUN" = "1" ]; then
      echo "[dry-run] stale socket: $sock"
    else
      rm -f -- "$sock" "$sock.lock"
      echo "removed stale socket: $sock"
    fi
    removed=$((removed + 1))
  done
done

if [ "$DRY_RUN" = "1" ]; then
  echo "[dry-run] $removed stale socket(s) found"
else
  echo "cleaned $removed stale socket(s)"
fi
