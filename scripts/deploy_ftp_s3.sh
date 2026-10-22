#!/usr/bin/env bash
# =============================================================================
# deploy_ftp_s3.sh — 在 CoreFusion 测试节点上部署 FTP(vsftpd)与 S3 兼容存储(MinIO)
#
# 部署说明见内部节点规划文档第 4 节
#   - FTP 节点(N5): 部署 vsftpd,被动模式,供 CoreFusion UploadToFTP 使用
#   - S3  节点(N6): 部署 MinIO single-drive,供 CoreFusion UploadToS3 使用
#
# 节点无外网时，通过 PANEL_PROXY 环境变量指定面板代理下载二进制。
# 用法:
#   ./deploy_ftp_s3.sh ftp            # 在 FTP 节点执行
#   ./deploy_ftp_s3.sh s3             # 在 S3 节点执行
#   ./deploy_ftp_s3.sh all            # 单机同时跑两者(测试用)
# =============================================================================
set -euo pipefail

PROXY="${PANEL_PROXY:-}"
if [[ -n "$PROXY" ]]; then
  export http_proxy="$PROXY" https_proxy="$PROXY" HTTP_PROXY="$PROXY" HTTPS_PROXY="$PROXY"
fi

# ---- 可覆盖的变量(通过环境变量传参) -----------------------------------------
FTP_USER="${FTP_USER:-ftpuser}"
FTP_PASS="${FTP_PASS:?请通过环境变量 FTP_PASS 设置密码}"
FTP_ROOT="${FTP_ROOT:-/srv/ftp}"
S3_USER="${S3_USER:-cf_s3}"
S3_PASS="${S3_PASS:?请通过环境变量 S3_PASS 设置密码}"
S3_DIR="${S3_DIR:-/srv/minio}"
S3_BUCKET="${S3_BUCKET:-corefusion-backup}"
DATA_DIR="${DATA_DIR:-/opt/corefusion/data}"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

# =============================================================================
# FTP 部署
# =============================================================================
deploy_ftp() {
  local ip
  ip="$(hostname -I | awk '{print $1}')"
  log "==> 部署 FTP (vsftpd) 监听 0.0.0.0:21, pasv_address=$ip"

  apt-get update -y
  apt-get install -y vsftpd lftp

  local conf="/etc/vsftpd.conf"
  cat > "$conf" <<EOF
listen=YES
listen_ipv6=NO
anonymous_enable=NO
local_enable=YES
write_enable=YES
chroot_local_user=YES
allow_writeable_chroot=YES
pasv_enable=YES
pasv_min_port=21100
pasv_max_port=21110
pasv_address=$ip
pasv_promiscuous=YES
local_root=$FTP_ROOT
xferlog_enable=YES
EOF

  mkdir -p "$FTP_ROOT/corefusion"
  if ! id "$FTP_USER" &>/dev/null; then
    useradd -m "$FTP_USER"
  fi
  echo "$FTP_USER:$FTP_PASS" | chpasswd
  chown -R "$FTP_USER:$FTP_USER" "$FTP_ROOT"

  systemctl restart vsftpd && systemctl enable vsftpd
  log "✅ FTP 已启动: ftp://$ip:21  用户=$FTP_USER"
  log "   被动端口段 21100-21110 需在防火墙放行(内网一般已通)"
}

# =============================================================================
# S3 (MinIO) 部署
# =============================================================================
deploy_s3() {
  local ip
  ip="$(hostname -I | awk '{print $1}')"
  log "==> 部署 MinIO(S3 兼容) $S3_DIR, 监听 $ip:9000 / 控制台 $ip:9001"

  mkdir -p "$S3_DIR" "$DATA_DIR"
  cd "$DATA_DIR"

  if [ ! -x ./minio ]; then
    log "下载 minio 二进制 ..."
    wget -q "https://dl.min.io/server/minio/release/linux-amd64/minio" -O minio
    chmod +x minio
  fi

  export MINIO_ROOT_USER="$S3_USER"
  export MINIO_ROOT_PASSWORD="$S3_PASS"

  # 用 systemd 托管(若可用),否则 nohup 兜底
  if command -v systemctl >/dev/null 2>&1; then
    cat > /etc/systemd/system/minio.service <<EOF
[Unit]
Description=MinIO (CoreFusion S3 backup target)
After=network.target

[Service]
Type=simple
Environment=MINIO_ROOT_USER=$S3_USER
Environment=MINIO_ROOT_PASSWORD=$S3_PASS
ExecStart=$DATA_DIR/minio server $S3_DIR --address $ip:9000 --console-address $ip:9001
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl restart minio
    systemctl enable minio
  else
    nohup ./"$DATA_DIR/minio" server "$S3_DIR" --address "$ip:9000" --console-address "$ip:9001" \
      > "$S3_DIR/minio.log" 2>&1 &
  fi

  # 等 minio 起来后建桶
  for i in $(seq 1 30); do
    if curl -sf "http://$ip:9000/minio/health/live" >/dev/null 2>&1; then break; fi
    sleep 1
  done

  if command -v mc >/dev/null 2>&1; then
    mc alias set local "http://$ip:9000" "$S3_USER" "$S3_PASS"
    mc mb -p "local/$S3_BUCKET"
    log "✅ MinIO 已启动, 桶 $S3_BUCKET 就绪: http://$ip:9000"
  else
    log "⚠️  mc 未安装, 请手动建桶: $S3_BUCKET (endpoint http://$ip:9000)"
    log "✅ MinIO 已启动: http://$ip:9000  控制台 http://$ip:9001"
  fi
}

# =============================================================================
main() {
  case "${1:-all}" in
    ftp) deploy_ftp ;;
    s3)  deploy_s3 ;;
    all) deploy_ftp; deploy_s3 ;;
    *) echo "用法: $0 [ftp|s3|all]"; exit 1 ;;
  esac
  log "部署完成。"
}

main "$@"
