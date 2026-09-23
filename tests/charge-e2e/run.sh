#!/usr/bin/env bash
#
# 充值链路**端到端**测试（配套同目录的独立测试模块 ./mockchargesite ✓）
#
# ⚠ 纯测试资产 ✓：产品代码里没有任何"为测试开的口子" ✓；
#   mockchargesite 是独立 go.mod 模块 ✓，不参与产品构建 ✓。
#
# 覆盖：管理员建站点 → 置 verified → 客户端下单 → 面板调模拟站点拿 payment_url
#      → 模拟站点回调（HMAC 签名）→ 余额到账 + 流水 + 订单 paid
# 反向用例：错签名 / 金额不符 / 时间过期 / 重放同一 nonce —— 都应被拒（400 ✓）
#
# 用法：
#   bash tests/charge-e2e/run.sh
#   PANEL_URL=http://127.0.0.1:8080 AMOUNT_YUAN=5 bash tests/charge-e2e/run.sh
#   KEEP=1 bash tests/charge-e2e/run.sh      # 跑完保留模拟站点，方便手动点收银台
#
# 环境变量：PANEL_URL / CF_DB / MOCK_PORT / ADMIN_USER / ADMIN_PASS / AMOUNT_YUAN / KEEP
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
PANEL="${PANEL_URL:-http://127.0.0.1:8080}"
DB="${CF_DB:-$ROOT/data/sqlite/app.db}"
PORT="${MOCK_PORT:-9099}"
MOCK_URL="http://127.0.0.1:$PORT"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin123}"
AMOUNT_YUAN="${AMOUNT_YUAN:-1}"
AMOUNT_FEN=$((AMOUNT_YUAN * 100))
KEEP="${KEEP:-}"

TMP="$(mktemp -d)"
PASS=0
FAIL=0
TOKEN=""
SITE_ID=""
CLIENT_ID=""
CLIENT_TOKEN=""
MOCK_PID=""
NEG_TRADE=""
SITE_SECRET="cs_$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
CLIENT_EMAIL="charge-e2e-$(date +%s)@test.local"
STEP=0

step() { STEP=$((STEP + 1)); printf '\n\033[1;36m[%d] %s\033[0m\n' "$STEP" "$1"; }
ok()   { PASS=$((PASS + 1)); printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { FAIL=$((FAIL + 1)); printf '  \033[31m✗ %s\033[0m\n' "$1"; }
info() { printf '    %s\n' "$1"; }

cleanup() {
  # ⚠ 中途失败也要清干净 ✗：测试站点/客户端统一在退出钩子里删 ✓
  #（否则前面一 exit 就留下测试数据 ✗ —— 实测踩到过 ✓）。
  if [[ -n "$TOKEN" ]]; then
    [[ -n "$SITE_ID" ]]   && curl -s -X DELETE "$PANEL/api/v1/charge-sites/$SITE_ID" -H "Authorization: Bearer $TOKEN" >/dev/null 2>&1
    [[ -n "$CLIENT_ID" ]] && curl -s -X DELETE "$PANEL/api/v1/clients/$CLIENT_ID" -H "Authorization: Bearer $TOKEN" >/dev/null 2>&1
  fi
  if [[ -n "$MOCK_PID" && -z "$KEEP" ]]; then kill "$MOCK_PID" 2>/dev/null; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# ---------------------------------------------------------------- helpers ----
# jget <json> <dotted.path> —— 取 JSON 字段（支持 a.b.c ✓）
jget() {
  printf '%s' "$1" | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    print(""); sys.exit(0)
for k in sys.argv[1].split("."):
    if isinstance(d, list):
        d = d[int(k)] if k.isdigit() and len(d) > int(k) else {}
    elif isinstance(d, dict):
        d = d.get(k)
    else:
        d = None
    if d is None:
        print(""); sys.exit(0)
print(d if not isinstance(d, (dict, list)) else json.dumps(d, ensure_ascii=False))
' "$2"
}

# --------------------------------------------------------------- preflight ---
step "预检"
command -v sqlite3 >/dev/null || { printf '  ✗ 缺少 sqlite3（无法置 verified）\n'; exit 1; }
command -v python3 >/dev/null || { printf '  ✗ 缺少 python3\n'; exit 1; }
command -v go      >/dev/null || { printf '  ✗ 缺少 go\n'; exit 1; }
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$PANEL/" || true)"
[[ "$CODE" == "200" ]] && ok "面板可达 ✓ $PANEL" || { bad "面板不可达（HTTP $CODE）→ 先启动面板"; exit 1; }
[[ -f "$DB" ]] && ok "数据库可读 ✓ $DB" || { bad "找不到数据库 $DB（用 CF_DB 指定）"; exit 1; }

# ------------------------------------------------------------ 管理员登录 -----
step "管理员登录"
LOGIN="$(curl -s -X POST "$PANEL/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")"
TOKEN="$(jget "$LOGIN" "data.access_token")"
[[ -n "$TOKEN" ]] && ok "拿到管理员 token ✓" || { bad "登录失败：$LOGIN"; exit 1; }
api() { # api <method> <path> [body]
  local m="$1" p="$2" b="${3:-}"
  if [[ -n "$b" ]]; then
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$b"
  else
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $TOKEN"
  fi
}

# ------------------------------------------------------------ 建充值站点 -----
step "创建充值站点（SiteURL 指向模拟站点）"
SITE_BODY="$(cat <<JSON
{"name":"mock-e2e-$(date +%s)","site_url":"$MOCK_URL","callback_secret":"$SITE_SECRET",
 "callback_whitelist":"","supported_methods":"[\"alipay\",\"wechat\"]","currency":"CNY",
 "enabled":true,"sort_order":0}
JSON
)"
SITE_RESP="$(api POST /api/v1/charge-sites "$SITE_BODY")"
SITE_ID="$(jget "$SITE_RESP" "data.site.id")"
RET_SECRET="$(jget "$SITE_RESP" "data.plaintext_secret")"
if [[ -z "$SITE_ID" ]]; then bad "建站点失败：$SITE_RESP"; exit 1; fi
ok "站点已创建 ✓ id=$SITE_ID"
[[ "$RET_SECRET" == "$SITE_SECRET" ]] && ok "回调密钥一致 ✓（面板回显的 plaintext_secret 与传入相同）" \
  || bad "回调密钥不一致 ✗（面板回显=$RET_SECRET）→ 回调会验签失败"

# ------------------------------------------------------- 置 verified（快路径）-
step "把站点置为已验证（测试快路径）"
# 正式流程要走域名验证（把 WW_verify_<token>.txt 放到站点根目录）；
# 本地模拟站点没有公网域名 → 直接改库最省事（回调前置校验要求 verified=true 且未过期）。
sqlite3 "$DB" "update charge_sites set verified=1, verify_expire_at=datetime('now','+365 days') where id=$SITE_ID;" \
  && ok "verified=1 ✓" || bad "置 verified 失败"
V="$(sqlite3 "$DB" "select verified from charge_sites where id=$SITE_ID;")"
[[ "$V" == "1" ]] && ok "库里确认 verified=$V ✓" || bad "库里 verified=$V ✗"

# --------------------------------------------------------- 启动模拟站点 -----
step "启动模拟充值站点（独立测试模块 ./mockchargesite ✓）"
( cd "$HERE/mockchargesite" && go build -o "$TMP/mockchargesite" . ) 2>"$TMP/build.log" \
  || { bad "编译模拟站点失败 ✗"; cat "$TMP/build.log"; exit 1; }
"$TMP/mockchargesite" -addr "127.0.0.1:$PORT" -secret "$SITE_SECRET" >"$TMP/mock.log" 2>&1 &
MOCK_PID=$!
for _ in $(seq 1 40); do
  curl -s -o /dev/null "$MOCK_URL/healthz" && break
  sleep 0.2
done
curl -s -o /dev/null "$MOCK_URL/healthz" && ok "模拟站点已就绪 ✓ $MOCK_URL（收银台 $MOCK_URL/orders）" \
  || { bad "模拟站点启动失败 ✗"; cat "$TMP/mock.log"; exit 1; }

# --- 回调签名小工具：与官方文档同一套算法（先算 body_hash、再把 sign 放进 body ✓）---
cat >"$TMP/sign.py" <<'EOF_PY'
import argparse, hashlib, hmac, json, time

p = argparse.ArgumentParser()
p.add_argument("--trade-no", required=True)
p.add_argument("--site-id", required=True)
p.add_argument("--amount", required=True, help="单位：元")
p.add_argument("--currency", default="CNY")
p.add_argument("--secret", required=True)
p.add_argument("--status", default="paid")
p.add_argument("--site-trade-no", default="E2ETEST")
p.add_argument("--ts-offset", type=int, default=0)
p.add_argument("--nonce", default="")
p.add_argument("--break-sign", action="store_true")
p.add_argument("--out", required=True)
p.add_argument("--panel", default="")
a = p.parse_args()

ts = int(time.time()) + a.ts_offset
nonce = a.nonce or hashlib.sha256(("%s%d" % (a.trade_no, ts)).encode()).hexdigest()[:32]
amount = int(a.amount)  # ⚠ 元（面板按 input.Amount*100 与订单的"分"比对）

body = {
    "trade_no": a.trade_no, "site_trade_no": a.site_trade_no, "status": a.status,
    "amount": amount, "currency": a.currency, "timestamp": ts, "nonce": nonce,
    "paid_currency": a.currency, "paid_amount": amount, "method": "alipay",
    "exchange_rate": 1.0, "paid_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
}
# 第 1 步：body_hash = SHA256(**加 sign 之前**的 body 字节) —— 与官方文档第 1 步一致
body_str = json.dumps(body, separators=(",", ":"), ensure_ascii=False)
body_hash = hashlib.sha256(body_str.encode()).hexdigest()
# 第 2~4 步：8 字段载荷（字典序紧凑）→ HMAC-SHA256(回调密钥)
sig_payload = {
    "amount": amount, "body_hash": body_hash, "currency": a.currency, "nonce": nonce,
    "site_trade_no": a.site_trade_no, "timestamp": ts, "trade_no": a.trade_no, "version": "v1",
}
sig_str = json.dumps(sig_payload, sort_keys=True, separators=(",", ":"))
sign = hmac.new(a.secret.encode(), sig_str.encode(), hashlib.sha256).hexdigest()
if a.break_sign:
    sign = "0" * 64
# 发送体：把 sign 追加进 body（面板侧剔除 sign 成员后重算 hash，位置无关）
final = json.dumps(dict(body, sign=sign), separators=(",", ":"), ensure_ascii=False)
with open(a.out, "w", encoding="utf-8") as f:
    f.write(final)
print("%s/api/v1/charge/callback/%s" % (a.panel.rstrip("/"), a.site_id))
EOF_PY

make_callback() { # 生成回调请求 → URL 打到 stdout、body 写到 $TMP/cb.json
  python3 "$TMP/sign.py" "$@" --out "$TMP/cb.json" --panel "$PANEL"
}

# ------------------------------------------------------------ 准备客户端 -----
step "准备测试客户端并登录"
PLAN_ID="$(sqlite3 "$DB" "select id from plan_templates order by id limit 1;")"
CLIENT_RESP="$(api POST /api/v1/clients "{\"email\":\"$CLIENT_EMAIL\",\"password\":\"Passw0rd!23\",\"plan_id\":${PLAN_ID:-1}}")"
CLIENT_ID="$(jget "$CLIENT_RESP" "data.id")"
if [[ -n "$CLIENT_ID" ]]; then
  ok "已创建测试客户端 ✓ id=$CLIENT_ID ($CLIENT_EMAIL)"
else
  bad "建客户端失败 → 后续下单会失败：$(printf '%s' "$CLIENT_RESP" | head -c 160)"
fi
CLIENT_LOGIN="$(curl -s -X POST "$PANEL/api/v1/client-auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$CLIENT_EMAIL\",\"password\":\"Passw0rd!23\"}")"
CLIENT_TOKEN="$(jget "$CLIENT_LOGIN" "data.access_token")"
[[ -n "$CLIENT_TOKEN" ]] && ok "客户端登录成功 ✓" || { bad "客户端登录失败：$CLIENT_LOGIN"; exit 1; }
capi() { # capi <method> <path> [body]
  local m="$1" p="$2" b="${3:-}"
  if [[ -n "$b" ]]; then
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $CLIENT_TOKEN" -H 'Content-Type: application/json' -d "$b"
  else
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $CLIENT_TOKEN"
  fi
}

# ------------------------------------------------------------ 下单 ----------
step "客户端下单（面板应调用模拟站点拿 payment_url）"
BAL_BEFORE="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
info "下单前余额：$BAL_BEFORE"
ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
TRADE_NO="$(jget "$ORDER" "data.trade_no")"
PAY_URL="$(jget "$ORDER" "data.payment_url")"
if [[ -z "$TRADE_NO" ]]; then bad "下单失败：$ORDER"; exit 1; fi
ok "下单成功 ✓ trade_no=$TRADE_NO"
# ⚠ 用**前缀比较**判断归属 ✓（别用正则抠端口 ✗：URL 后面还有 /pay?trade_no=… ✗）
[[ -n "$PAY_URL" && "$PAY_URL" == "$MOCK_URL/"* ]] && ok "payment_url 指向模拟站点 ✓ $PAY_URL" \
  || bad "payment_url 不对 ✗ $PAY_URL（期望以 $MOCK_URL/ 开头 → 出站调用可能没走模拟站点）"
[[ "$(sqlite3 "$DB" "select count(*) from charge_orders where trade_no='$TRADE_NO';")" == "1" ]] \
  && ok "订单已入库 ✓" || bad "订单未入库 ✗"

# --------------------------------------------------------- 支付 + 到账 ------
step "模拟支付（由模拟站点回调面板）"
PAY_RESP="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$TRADE_NO&status=paid")"
PANEL_CODE="$(jget "$PAY_RESP" "panel_code")"
if [[ "$PANEL_CODE" == "200" ]]; then
  ok "回调被受理 ✓ 面板响应：$(jget "$PAY_RESP" "panel_body" | head -c 140)"
else
  bad "回调失败 ✗ $PAY_RESP"
  info "模拟站点日志："; tail -6 "$TMP/mock.log" | sed 's/^/      /'
fi
BAL_AFTER="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
info "下单后余额：$BAL_AFTER"
if [[ -n "$BAL_BEFORE" && -n "$BAL_AFTER" ]]; then
  DELTA=$((BAL_AFTER - BAL_BEFORE))
  [[ "$DELTA" == "$AMOUNT_FEN" ]] && ok "余额增加 $DELTA 分 ✓（期望 $AMOUNT_FEN 分）" \
    || bad "余额变化不对 ✗ got=$DELTA want=$AMOUNT_FEN"
fi
TX="$(capi GET /api/v1/client-auth/balance/transactions)"
printf '%s' "$TX" | grep -q '"charge"' && ok "流水里有 charge 记录 ✓" || bad "没看到 charge 流水 ✗ $TX"
ST="$(sqlite3 "$DB" "select status from charge_orders where trade_no='$TRADE_NO';")"
[[ "$ST" == "paid" ]] && ok "订单状态 = paid ✓" || bad "订单状态 = $ST ✗"
info "回调次数 = $(sqlite3 "$DB" "select callback_count from charge_orders where trade_no='$TRADE_NO';")（1 次正常；重放会被幂等挡掉）"

# ---------------------------------------------------------- 反向用例 --------
step "反向用例（都应被拒）"
neg() { # neg <名称> <期望http> <附加签名参数...>
  local name="$1" want="$2"; shift 2
  local url code
  url="$(make_callback --trade-no "$NEG_TRADE" --site-id "$SITE_ID" --secret "$SITE_SECRET" "$@")"
  code="$(curl -s -o "$TMP/neg.out" -w '%{http_code}' -X POST "$url" \
    -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
  if [[ "$code" == "$want" ]]; then ok "$name → HTTP $code ✓"
  else bad "$name → HTTP $code ✗（期望 $want）$(head -c 160 "$TMP/neg.out")"; fi
}
NEG_ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
NEG_TRADE="$(jget "$NEG_ORDER" "data.trade_no")"
if [[ -n "$NEG_TRADE" ]]; then
  neg "错误签名"            400 --amount "$AMOUNT_YUAN" --break-sign
  neg "金额不符"            400 --amount "$((AMOUNT_YUAN + 7))"
  neg "时间过期（±300s 外）" 400 --amount "$AMOUNT_YUAN" --ts-offset -3600
  RP_NONCE="replay$(date +%s%N)"
  neg "正确回调（给重放用）"  200 --amount "$AMOUNT_YUAN" --nonce "$RP_NONCE"
  neg "重放同一 nonce"      400 --amount "$AMOUNT_YUAN" --nonce "$RP_NONCE"
else
  bad "反向用例的订单创建失败 ✗ $NEG_ORDER"
fi

# ------------------------------------------------------------ 清理 ----------
step "清理"
ok "测试站点与测试客户端由退出钩子统一删除 ✓（site=$SITE_ID client=$CLIENT_ID）"
if [[ -z "$KEEP" ]]; then info "模拟站点已停止 ✓（KEEP=1 可保留）"; else info "模拟站点保留在 $MOCK_URL ✓（收银台 $MOCK_URL/orders）"; fi

printf '\n\033[1m========== 汇总：通过 %d ✓ / 失败 %d ✗ ==========\033[0m\n' "$PASS" "$FAIL"
[[ "$FAIL" == "0" ]] && exit 0 || exit 1
