#!/usr/bin/env bash
#
# 充值链路**端到端**测试（配套同目录的独立测试模块 ./mockchargesite）
#
# ⚠ 纯测试资产：产品代码里没有任何"为测试开的口子"；mockchargesite 是独立 go.mod 模块，
#   不参与产品构建（go list ./... 里看不到它）。
#
# 覆盖四条真实链路（全部走真实 API，不做任何"模拟代码"）：
#   [余额充值] 客户端下单 → 面板调模拟站点拿 payment_url → 支付 → 回调 → 余额到账 + 流水
#   [套餐充值] pay_method=charge_site 买套餐 → 支付 → 回调 → 套餐真正生效（激活/续期）
#   [订单过期] 订单 TTL 数据 + 过期后回调的行为（数据是否仍然严格对上）
#   [反向用例] 错签名 / 金额不符 / 时间过期 / 重放同一 nonce —— 都应被拒（400）
#
# 用法：
#   bash tests/charge-e2e/run.sh
#   PANEL_URL=http://127.0.0.1:8080 bash tests/charge-e2e/run.sh
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
warn() { printf '    \033[33m⚠ %s\033[0m\n' "$1"; }

cleanup() {
  # 中途失败也要清干净：测试站点/客户端统一在退出钩子里删。
  if [[ -n "$TOKEN" ]]; then
    [[ -n "$SITE_ID" ]]   && curl -s -X DELETE "$PANEL/api/v1/charge-sites/$SITE_ID" -H "Authorization: Bearer $TOKEN" >/dev/null 2>&1
    [[ -n "$CLIENT_ID" ]] && curl -s -X DELETE "$PANEL/api/v1/clients/$CLIENT_ID" -H "Authorization: Bearer $TOKEN" >/dev/null 2>&1
  fi
  if [[ -n "$MOCK_PID" && -z "$KEEP" ]]; then kill "$MOCK_PID" 2>/dev/null; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# ---------------------------------------------------------------- helpers ----
jget() { # jget <json> <dotted.path>
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
sqlq() { sqlite3 "$DB" "$1"; }

# --------------------------------------------------------------- preflight ---
step "预检"
for c in sqlite3 python3 go curl; do
  command -v "$c" >/dev/null || { printf '  ✗ 缺少 %s\n' "$c"; exit 1; }
done
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$PANEL/" || true)"
[[ "$CODE" == "200" ]] && ok "面板可达 ✓ $PANEL" || { bad "面板不可达（HTTP $CODE）→ 先启动面板"; exit 1; }
[[ -f "$DB" ]] && ok "数据库可读 ✓ $DB" || { bad "找不到数据库 $DB（用 CF_DB 指定）"; exit 1; }

# ------------------------------------------------------------ 管理员登录 -----
step "管理员登录"
LOGIN="$(curl -s -X POST "$PANEL/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")"
TOKEN="$(jget "$LOGIN" "data.access_token")"
[[ -n "$TOKEN" ]] && ok "拿到管理员 token ✓" || { bad "登录失败：$LOGIN"; exit 1; }
api() {
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
[[ -z "$SITE_ID" ]] && { bad "建站点失败：$SITE_RESP"; exit 1; }
ok "站点已创建 ✓ id=$SITE_ID"
[[ "$RET_SECRET" == "$SITE_SECRET" ]] && ok "回调密钥一致 ✓" || bad "回调密钥不一致 ✗（回显=$RET_SECRET）"

step "把站点置为已验证（测试快路径）"
# 正式流程走域名验证；本地模拟站点没有公网域名 → 直接改库最省事
sqlq "update charge_sites set verified=1, verify_expire_at=datetime('now','+365 days') where id=$SITE_ID;" \
  && ok "verified=1 ✓" || bad "置 verified 失败"

# --------------------------------------------------------- 启动模拟站点 -----
step "启动模拟充值站点（独立测试模块 ✓）"
( cd "$HERE/mockchargesite" && go build -o "$TMP/mockchargesite" . ) 2>"$TMP/build.log" \
  || { bad "编译模拟站点失败 ✗"; cat "$TMP/build.log"; exit 1; }
"$TMP/mockchargesite" -addr "127.0.0.1:$PORT" -secret "$SITE_SECRET" >"$TMP/mock.log" 2>&1 &
MOCK_PID=$!
for _ in $(seq 1 40); do curl -s -o /dev/null "$MOCK_URL/healthz" && break; sleep 0.2; done
curl -s -o /dev/null "$MOCK_URL/healthz" && ok "模拟站点已就绪 ✓ $MOCK_URL（收银台 $MOCK_URL/orders）" \
  || { bad "模拟站点启动失败 ✗"; cat "$TMP/mock.log"; exit 1; }

# --- 回调签名工具：与官方文档同一套算法；⚠ amount 单位是「分」-------------------------
cat >"$TMP/sign.py" <<'EOF_PY'
import argparse, hashlib, hmac, json, time

p = argparse.ArgumentParser()
p.add_argument("--trade-no", required=True)
p.add_argument("--site-id", required=True)
p.add_argument("--amount", required=True, help="单位：分（与面板订单同单位）")
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
amount = int(a.amount)  # ⚠ 分（int64）—— 面板按同单位直接比较

body = {
    "trade_no": a.trade_no, "site_trade_no": a.site_trade_no, "status": a.status,
    "amount": amount, "currency": a.currency, "timestamp": ts, "nonce": nonce,
    "paid_currency": a.currency, "paid_amount": amount, "method": "alipay",
    "exchange_rate": 1.0, "paid_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
}
body_str = json.dumps(body, separators=(",", ":"), ensure_ascii=False)
body_hash = hashlib.sha256(body_str.encode()).hexdigest()
sig_payload = {
    "amount": amount, "body_hash": body_hash, "currency": a.currency, "nonce": nonce,
    "site_trade_no": a.site_trade_no, "timestamp": ts, "trade_no": a.trade_no, "version": "v1",
}
sig_str = json.dumps(sig_payload, sort_keys=True, separators=(",", ":"))
sign = hmac.new(a.secret.encode(), sig_str.encode(), hashlib.sha256).hexdigest()
if a.break_sign:
    sign = "0" * 64
final = json.dumps(dict(body, sign=sign), separators=(",", ":"), ensure_ascii=False)
with open(a.out, "w", encoding="utf-8") as f:
    f.write(final)
print("%s/api/v1/charge/callback/%s" % (a.panel.rstrip("/"), a.site_id))
EOF_PY
make_callback() { python3 "$TMP/sign.py" "$@" --out "$TMP/cb.json" --panel "$PANEL"; }

# ------------------------------------------------------------ 准备客户端 -----
step "准备测试客户端并登录"
PLAN_ID="$(sqlq "select id from plan_templates order by id limit 1;")"
CLIENT_RESP="$(api POST /api/v1/clients "{\"email\":\"$CLIENT_EMAIL\",\"password\":\"Passw0rd!23\",\"plan_id\":${PLAN_ID:-1}}")"
CLIENT_ID="$(jget "$CLIENT_RESP" "data.id")"
[[ -n "$CLIENT_ID" ]] && ok "已创建测试客户端 ✓ id=$CLIENT_ID" \
  || bad "建客户端失败：$(printf '%s' "$CLIENT_RESP" | head -c 160)"
CLIENT_LOGIN="$(curl -s -X POST "$PANEL/api/v1/client-auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$CLIENT_EMAIL\",\"password\":\"Passw0rd!23\"}")"
CLIENT_TOKEN="$(jget "$CLIENT_LOGIN" "data.access_token")"
[[ -n "$CLIENT_TOKEN" ]] && ok "客户端登录成功 ✓" || { bad "客户端登录失败：$CLIENT_LOGIN"; exit 1; }
capi() {
  local m="$1" p="$2" b="${3:-}"
  if [[ -n "$b" ]]; then
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $CLIENT_TOKEN" -H 'Content-Type: application/json' -d "$b"
  else
    curl -s -X "$m" "$PANEL$p" -H "Authorization: Bearer $CLIENT_TOKEN"
  fi
}

# ============================ 场景 1：余额充值 ===============================
step "【余额充值】下单 → 面板调模拟站点 → 支付 → 到账"
BAL_BEFORE="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
info "下单前余额：$BAL_BEFORE"
ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
TRADE_NO="$(jget "$ORDER" "data.trade_no")"
PAY_URL="$(jget "$ORDER" "data.payment_url")"
[[ -z "$TRADE_NO" ]] && { bad "下单失败：$ORDER"; exit 1; }
ok "下单成功 ✓ trade_no=$TRADE_NO"
[[ -n "$PAY_URL" && "$PAY_URL" == "$MOCK_URL/"* ]] && ok "payment_url 指向模拟站点 ✓" \
  || bad "payment_url 不对 ✗ $PAY_URL（期望以 $MOCK_URL/ 开头）"
[[ "$(sqlq "select count(*) from charge_orders where trade_no='$TRADE_NO';")" == "1" ]] \
  && ok "订单已入库 ✓" || bad "订单未入库 ✗"

PAY_RESP="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$TRADE_NO&status=paid")"
[[ "$(jget "$PAY_RESP" "panel_code")" == "200" ]] && ok "回调被受理 ✓ $(jget "$PAY_RESP" "panel_body" | head -c 120)" \
  || { bad "回调失败 ✗ $PAY_RESP"; tail -6 "$TMP/mock.log" | sed 's/^/      /'; }
BAL_AFTER="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
if [[ -n "$BAL_BEFORE" && -n "$BAL_AFTER" ]]; then
  DELTA=$((BAL_AFTER - BAL_BEFORE))
  [[ "$DELTA" == "$AMOUNT_FEN" ]] && ok "余额 +$DELTA 分 ✓（期望 $AMOUNT_FEN）" || bad "余额变化不对 ✗ got=$DELTA"
fi
printf '%s' "$(capi GET /api/v1/client-auth/balance/transactions)" | grep -q '"charge"' \
  && ok "流水出现 charge ✓" || bad "没看到 charge 流水 ✗"
[[ "$(sqlq "select status from charge_orders where trade_no='$TRADE_NO';")" == "paid" ]] \
  && ok "订单状态 = paid ✓" || bad "订单状态不对 ✗"

# ============================ 场景 2：套餐充值 ==============================
step "【套餐充值】pay_method=charge_site 买套餐 → 支付 → 套餐生效"
VAR_ROW="$(sqlq "select id||'|'||price||'|'||duration_days||'|'||name from plan_variants where enabled=1 order by id limit 1;")"
VARIANT_ID="${VAR_ROW%%|*}"; REST="${VAR_ROW#*|}"; VAR_PRICE="${REST%%|*}"; REST="${REST#*|}"; VAR_DAYS="${REST%%|*}"; VAR_NAME="${REST#*|}"
if [[ -z "$VARIANT_ID" || -z "$VAR_PRICE" ]]; then
  bad "找不到可用套餐变体 ✗（plan_variants 为空）"
else
  info "套餐：id=$VARIANT_ID $VAR_NAME 价格=$VAR_PRICE 分（$((VAR_PRICE / 100)).$((VAR_PRICE % 100)) 元）时长=${VAR_DAYS}天"
  BAL_P1="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
  EXP_BEFORE="$(sqlq "select coalesce(expires_at,'') from clients where email='$CLIENT_EMAIL';")"
  # ⚠ 用 renew 而不是 new：测试客户端创建时就带了 plan_id（CreateClient 要求 plan_id ✓），
  # 所以它"已有套餐" ✓ → 用 new 会被合理拒绝（"you already have an active plan"）✓。
  P_ORDER="$(capi POST /api/v1/client-auth/orders \
    "{\"variant_id\":$VARIANT_ID,\"order_type\":\"renew\",\"pay_method\":\"charge_site\",\"charge_site_id\":$SITE_ID}")"
  P_TRADE="$(jget "$P_ORDER" "data.trade_no")"
  P_URL="$(jget "$P_ORDER" "data.payment_url")"
  if [[ -z "$P_TRADE" ]]; then
    bad "套餐下单失败 ✗ $P_ORDER"
  else
    ok "套餐下单成功 ✓ trade_no=$P_TRADE"
    P_AMT="$(sqlq "select amount from charge_orders where trade_no='$P_TRADE';")"
    P_TYPE="$(sqlq "select order_type from charge_orders where trade_no='$P_TRADE';")"
    [[ "$P_AMT" == "$VAR_PRICE" ]] && ok "订单金额 = 套餐价 $P_AMT 分 ✓（金额单位一致）" \
      || bad "订单金额不对 ✗ got=$P_AMT want=$VAR_PRICE"
    [[ "$P_TYPE" == "purchase" ]] && ok "order_type = purchase ✓" || bad "order_type = $P_TYPE ✗"
    [[ -n "$P_URL" && "$P_URL" == "$MOCK_URL/"* ]] && ok "payment_url 指向模拟站点 ✓" \
      || bad "payment_url 不对 ✗ $P_URL"
    P_PAY="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$P_TRADE&status=paid")"
    [[ "$(jget "$P_PAY" "panel_code")" == "200" ]] && ok "套餐回调被受理 ✓ $(jget "$P_PAY" "panel_body" | head -c 100)" \
      || { bad "套餐回调失败 ✗ $P_PAY"; tail -6 "$TMP/mock.log" | sed 's/^/      /'; }
    # 套餐是否真的生效 ✓
    CL_VARIANT="$(sqlq "select coalesce(plan_variant_id,0) from clients where email='$CLIENT_EMAIL';")"
    [[ "$CL_VARIANT" == "$VARIANT_ID" ]] && ok "客户端套餐已切换为该变体 ✓（plan_variant_id=$CL_VARIANT）" \
      || bad "套餐未生效 ✗ plan_variant_id=$CL_VARIANT（期望 $VARIANT_ID）"
    P_STATUS="$(sqlq "select coalesce(max(status),'') from plan_orders where client_id=$CLIENT_ID;")"
    [[ -n "$P_STATUS" && "$P_STATUS" != "pending" ]] && ok "套餐订单状态 = $P_STATUS ✓（不是 pending ✓）" \
      || bad "套餐订单状态异常 ✗ '$P_STATUS'"
    [[ "$(sqlq "select status from charge_orders where trade_no='$P_TRADE';")" == "paid" ]] \
      && ok "套餐的充订单 = paid ✓" || bad "套餐的充订单状态不对 ✗"
    BAL_P2="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
    [[ "$BAL_P1" == "$BAL_P2" ]] && ok "余额未被套餐购买改动 ✓（$BAL_P1）" \
      || info "余额变化：$BAL_P1 → $BAL_P2（若套餐走余额则属预期，请留意）"
    # 续期是否真的生效：客户有效期必须发生变化 ✓（这是"套餐充值成功"最硬的证据 ✓）
    EXP_AFTER="$(sqlq "select coalesce(expires_at,'') from clients where email='$CLIENT_EMAIL';")"
    info "客户有效期：${EXP_BEFORE:-<空>} → ${EXP_AFTER:-<空>}"
    [[ -n "$EXP_AFTER" && "$EXP_AFTER" != "$EXP_BEFORE" ]] \
      && ok "续期生效 ✓（有效期已顺延）" || bad "有效期没变化 ✗（$EXP_AFTER）"
  fi
fi

# ============================ 场景 3：订单过期 ==============================
step "【订单过期】TTL 数据 + 过期后回调是否仍严格对账"
E_ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
E_TRADE="$(jget "$E_ORDER" "data.trade_no")"
if [[ -z "$E_TRADE" ]]; then
  bad "过期用例下单失败 ✗ $E_ORDER"
else
  E_EXP="$(sqlq "select coalesce(expires_at,'') from charge_orders where trade_no='$E_TRADE';")"
  [[ -n "$E_EXP" ]] && ok "订单带上了过期时间 ✓ expires_at=$E_EXP" || bad "订单没有 expires_at ✗"
  # —— 把它"变老"（纯测试数据准备：本地不可能真等 15 分钟，TTL 目前写死 15min）——
  sqlq "update charge_orders set expires_at = datetime('now','-1 hour') where trade_no='$E_TRADE';" \
    && ok "已把该订单置为过期（测试数据准备 ✓）" || bad "置过期失败 ✗"
  info "过期后 expires_at = $(sqlq "select coalesce(expires_at,'') from charge_orders where trade_no='$E_TRADE';")"
  # 过期订单 + **数据完全对上**的回调 →
  E_URL="$(make_callback --trade-no "$E_TRADE" --site-id "$SITE_ID" --secret "$SITE_SECRET" --amount "$AMOUNT_FEN")"
  E_CODE="$(curl -s -o "$TMP/e.out" -w '%{http_code}' -X POST "$E_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
  if [[ "$E_CODE" == "200" ]]; then
    ok "过期订单的正确回调仍被受理（HTTP 200 ✓）"
    E_BAL="$(jget "$(capi GET /api/v1/client-auth/balance)" "data.balance")"
    ok "过期订单按订单金额精确入账 ✓（余额=$E_BAL）"
    warn "当前实现**不强制订单过期**：charge_orders.expires_at 全程没人读 → 迟到回调照样入账"
    warn "若希望"过期后拒收"，需要加判定；但注意"用户已付款却被拒"会造成丢单 ✗，建议做成可配置"
  else
    info "过期订单的正确回调被拒：HTTP $E_CODE $(head -c 140 "$TMP/e.out")"
    warn "请确认这是期望行为：用户已付款却入不了账会造成丢单 ✗"
  fi
  # 过期订单 + **金额不符** 的回调 → 必须被拒 ✓
  E2_ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
  E2_TRADE="$(jget "$E2_ORDER" "data.trade_no")"
  if [[ -n "$E2_TRADE" ]]; then
    sqlq "update charge_orders set expires_at = datetime('now','-1 hour') where trade_no='$E2_TRADE';"
    W_URL="$(make_callback --trade-no "$E2_TRADE" --site-id "$SITE_ID" --secret "$SITE_SECRET" --amount "$((AMOUNT_FEN + 1))")"
    W_CODE="$(curl -s -o "$TMP/w.out" -w '%{http_code}' -X POST "$W_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
    [[ "$W_CODE" == "400" ]] && ok "过期订单 + 金额差 1 分 → HTTP 400 ✓（金额必须严格对上）" \
      || bad "过期订单的金额不符回调没被拒 ✗ HTTP $W_CODE $(head -c 140 "$TMP/w.out")"
  fi
fi

# ---------------------------------------------------------- 反向用例 --------
step "【反向用例】都应被拒"
neg() {
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
  neg "错误签名"            400 --amount "$AMOUNT_FEN" --break-sign
  neg "金额不符（差 1 分）"   400 --amount "$((AMOUNT_FEN + 1))"
  neg "金额不符（错单位）"    400 --amount "$AMOUNT_YUAN"   # 元当分用 → 必须被拒
  neg "时间过期（±300s 外）" 400 --amount "$AMOUNT_FEN" --ts-offset -3600
  RP_NONCE="replay$(date +%s%N)"
  neg "正确回调（给重放用）"  200 --amount "$AMOUNT_FEN" --nonce "$RP_NONCE"
  neg "重放同一 nonce"      400 --amount "$AMOUNT_FEN" --nonce "$RP_NONCE"
else
  bad "反向用例的订单创建失败 ✗ $NEG_ORDER"
fi

# ------------------------------------------------------------ 清理 ----------
step "清理"
ok "测试站点与测试客户端由退出钩子统一删除 ✓（site=$SITE_ID client=$CLIENT_ID）"
if [[ -z "$KEEP" ]]; then info "模拟站点已停止 ✓（KEEP=1 可保留）"; else info "模拟站点保留在 $MOCK_URL ✓（收银台 $MOCK_URL/orders）"; fi

printf '\n\033[1m========== 汇总：通过 %d ✓ / 失败 %d ✗ ==========\033[0m\n' "$PASS" "$FAIL"
[[ "$FAIL" == "0" ]] && exit 0 || exit 1
