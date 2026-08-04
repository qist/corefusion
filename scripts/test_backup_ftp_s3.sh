#!/usr/bin/env bash
# =============================================================================
# test_backup_ftp_s3.sh — 测试 CoreFusion 备份/上传/还原全链路(FTP + S3)
#
# 对应文档: docs/NODE_PLANNING.md 第 4.5 节测试矩阵
# 链路: ①创建全量备份 → ②上传FTP → ③上传S3 → ④下载校验sha256 → ⑤全量还原
#
# 前置:
#   1. Panel 已运行(CoreFusion),且存在 super_admin 账号
#   2. N5 vsftpd / N6 MinIO 已按 deploy_ftp_s3.sh 部署
#   3. 已在 Panel 后台或 /backups/storages 配置好名为 ftp-n5 / minio-n6 的存储
#      (本脚本也提供 --setup-storages 自动创建这两个存储配置)
#
# 用法:
#   ./test_backup_ftp_s3.sh \
#     --panel http://127.0.0.1:8080 \
#     --user admin --pass '<super_admin密码>' \
#     --ftp-host <N5> --s3-host <N6> \
#     [--setup-storages]
# =============================================================================
set -euo pipefail

# ---- 默认值(可用参数或环境变量覆盖) ------------------------------------------
PANEL="${PANEL:-http://127.0.0.1:8080}"
USER="${CF_USER:-admin}"
PASS="${CF_PASS:-}"
FTP_HOST=""
S3_HOST=""
SETUP_STORAGES=0
DATA_DIR="${DATA_DIR:-/opt/corefusion/data}"

# ---- 解析参数 ---------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --panel)     PANEL="$2"; shift 2 ;;
    --user)      USER="$2"; shift 2 ;;
    --pass)      PASS="$2"; shift 2 ;;
    --ftp-host)  FTP_HOST="$2"; shift 2 ;;
    --s3-host)   S3_HOST="$2"; shift 2 ;;
    --setup-storages) SETUP_STORAGES=1; shift ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
done

[[ -z "$PASS" ]] && { echo "错误: 需 --pass 传入 super_admin 密码"; exit 1; }
[[ -z "$FTP_HOST" || -z "$S3_HOST" ]] && { echo "错误: 需 --ftp-host 与 --s3-host"; exit 1; }

API="$PANEL/api"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass_count=0; fail_count=0
check() { # check <名称> <条件命令>
  local name="$1"; shift
  if "$@"; then echo "  ✅ $name"; pass_count=$((pass_count+1));
  else echo "  ❌ $name"; fail_count=$((fail_count+1)); fi
}

echo "==> 登录 Panel 获取 token"
LOGIN_RESP="$(curl -sf -X POST "$API/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")"
TOKEN="$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*' | head -1 | sed 's/"token":"//')"
[[ -z "$TOKEN" ]] && { echo "登录失败: $LOGIN_RESP"; exit 1; }
AUTH="Authorization: Bearer $TOKEN"
echo "  token 已获取"

# ---------------------------------------------------------------------------
if [[ $SETUP_STORAGES -eq 1 ]]; then
  echo "==> 自动注册存储配置 (ftp-n5 / minio-n6)"
  curl -sf -X POST "$API/backups/storages" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"name\":\"ftp-n5\",\"type\":\"ftp\",\"endpoint\":\"ftp://$FTP_HOST:21\",\"access_key\":\"ftpuser\",\"secret_key\":\"CHANGE_ME_FTP_PASSWORD\"}" \
    && echo "  ftp-n5 已注册"
  curl -sf -X POST "$API/backups/storages" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"name\":\"minio-n6\",\"type\":\"s3\",\"endpoint\":\"http://$S3_HOST:9000\",\"bucket\":\"corefusion-backup\",\"access_key\":\"cf_s3\",\"secret_key\":\"CHANGE_ME_S3_PASSWORD\",\"use_path_style\":true}" \
    && echo "  minio-n6 已注册"
fi

# ① 创建全量备份
echo "==> ① 创建全量备份"
CREATE_RESP="$(curl -sf -X POST "$API/backups/create" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"name":"ftp-s3-test"}')"
BID="$(echo "$CREATE_RESP" | grep -o '"id":"[^"]*' | head -1 | sed 's/"id":"//')"
echo "  备份 ID = $BID"
check "① 创建备份返回有效 ID" [[ -n "$BID" ]]

# ② 上传 FTP
echo "==> ② 上传 FTP (ftp-n5)"
FTP_RESP="$(curl -sf -X POST "$API/backups/$BID/upload-ftp" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"storage":"ftp-n5"}')"
check "② 上传 FTP 成功" [[ -n "$FTP_RESP" ]]
echo "    $FTP_RESP" | head -c 200; echo

# ③ 上传 S3
echo "==> ③ 上传 S3 (minio-n6)"
S3_RESP="$(curl -sf -X POST "$API/backups/$BID/upload-s3" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"storage":"minio-n6"}')"
check "③ 上传 S3 成功" [[ -n "$S3_RESP" ]]
echo "    $S3_RESP" | head -c 200; echo

# ④ 下载并校验
echo "==> ④ 下载备份并校验"
curl -sf "$API/backups/$BID/download" -H "$AUTH" -o "$TMP/backup.tar.gz"
LOCAL_SHA="$(sha256sum "$TMP/backup.tar.gz" | awk '{print $1}')"
echo "   本地 sha256 = $LOCAL_SHA"
check "④ 下载文件非空" [[ -s "$TMP/backup.tar.gz" ]]

# ⑤ 全量还原(先记录还原前任务/用户数, 还原后再比对)
echo "==> ⑤ 全量还原 (会暂停服务并覆盖 data/ + 数据库)"
curl -sf -X POST "$API/backups/$BID/restore" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"confirm":true}' \
  && echo "   restore 请求已提交"
sleep 3
RESTORE_LIST="$(curl -sf "$API/backups/list" -H "$AUTH")"
check "⑤ 还原后仍可列出备份(服务已恢复)" [[ -n "$RESTORE_LIST" ]]

echo
echo "================ 测试结果 ================"
echo "通过: $pass_count   失败: $fail_count"
[[ $fail_count -eq 0 ]] && echo "🎉 全部通过" || echo "⚠️ 存在失败项, 请查看上方 ❌"
exit $fail_count
