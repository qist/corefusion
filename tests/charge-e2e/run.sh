#!/usr/bin/env bash
#
# 充值链路**端到端**测试（配套同目录的独立测试模块 ./mockchargesite）
#
# ⚠ 纯测试资产：产品代码里没有任何"为测试开的口子"；mockchargesite 是独立 go.mod 模块，
#   不参与产品构建（go list ./... 里看不到它）。
#
# 覆盖（全部走真实 API，不做任何"模拟代码"）：
#   [直接充值] 两笔：整元 + 非整元(9.90 元) —— 金额单位是否统一的关键证据，要求分毫不差
#   [套餐购买] 两条：renew（已有套餐续期）+ new（首次购买）—— 要求套餐真的生效、余额不被改动
#   [订单过期] TTL 数据 + 过期后回调是否仍严格对账（按产品决策：过期不强制）
#   [反向用例] 错签名 / 金额不符 / 错单位 / 时间过期 / 重放同一 nonce —— 都应被拒（400）
#
# 用法：
#   bash tests/charge-e2e/run.sh
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
ODD_FEN=990 # 9.90 元：非整元，专门用来验金额单位
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
note() { printf '    \033[33m· %s\033[0m\n' "$1"; }

cleanup() {
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
yuan() { printf '%d.%02d' $(($1 / 100)) $(($1 % 100)); }

# --------------------------------------------------------------- preflight ---
step "预检"
for c in sqlite3 python3 go curl; do
  command -v "$c" >/dev/null || { printf '  ✗ 缺少 %s\n' "$c"; exit 1; }
done
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$PANEL/" || true)"
[[ "$CODE" == "200" ]] && ok "面板可达 ✓ $PANEL" || { bad "面板不可达（HTTP $CODE）→ 先启动面板"; exit 1; }
[[ -f "$DB" ]] && ok "数据库可读 ✓ $DB" || { bad "找不到数据库 $DB（用 CF_DB 指定）"; exit 1; }

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
sqlq "update charge_sites set verified=1, verify_expire_at=datetime('now','+365 days') where id=$SITE_ID;" \
  && ok "verified=1 ✓" || bad "置 verified 失败"

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
amount = int(a.amount)

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
balance() { jget "$(capi GET /api/v1/client-auth/balance)" "data.balance"; }

# ==================== 场景 1：直接充值（余额充值）============================
step "【直接充值】整元 + 非整元各一笔（金额单位是否统一的关键证据）"
for SPEC in "整元($(yuan $AMOUNT_FEN)):$AMOUNT_FEN" "非整元($(yuan $ODD_FEN)):$ODD_FEN"; do
  LABEL="${SPEC%%:*}"; FEN="${SPEC##*:}"
  BAL_BEFORE="$(balance)"
  ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$FEN}")"
  TRADE_NO="$(jget "$ORDER" "data.trade_no")"
  PAY_URL="$(jget "$ORDER" "data.payment_url")"
  if [[ -z "$TRADE_NO" ]]; then bad "[$LABEL] 下单失败：$ORDER"; continue; fi
  ok "[$LABEL] 下单成功 ✓ trade_no=$TRADE_NO"
  [[ -n "$PAY_URL" && "$PAY_URL" == "$MOCK_URL/"* ]] && ok "[$LABEL] payment_url 指向模拟站点 ✓" \
    || bad "[$LABEL] payment_url 不对 ✗ $PAY_URL"
  O_AMT="$(sqlq "select amount from charge_orders where trade_no='$TRADE_NO';")"
  [[ "$O_AMT" == "$FEN" ]] && ok "[$LABEL] 订单金额 = $O_AMT 分 ✓" \
    || bad "[$LABEL] 订单金额 = $O_AMT ✗（期望 $FEN 分）"
  PAY_RESP="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$TRADE_NO&status=paid")"
  [[ "$(jget "$PAY_RESP" "panel_code")" == "200" ]] && ok "[$LABEL] 回调被受理 ✓" \
    || { bad "[$LABEL] 回调失败 ✗ $PAY_RESP"; tail -6 "$TMP/mock.log" | sed 's/^/      /'; }
  BAL_AFTER="$(balance)"
  DELTA=$((BAL_AFTER - BAL_BEFORE))
  [[ "$DELTA" == "$FEN" ]] && ok "[$LABEL] 余额 +$DELTA 分 ✓（分毫不差）" \
    || bad "[$LABEL] 余额变化 ✗ got=$DELTA want=$FEN"
  L_AMT="$(sqlq "select coalesce(sum(amount),0) from balance_transactions where related_charge_order_id=(select id from charge_orders where trade_no='$TRADE_NO') and type='charge';")"
  [[ "$L_AMT" == "$FEN" ]] && ok "[$LABEL] 流水金额 = $L_AMT 分 ✓" \
    || bad "[$LABEL] 流水金额 = $L_AMT ✗（期望 $FEN 分）"
  [[ "$(sqlq "select status from charge_orders where trade_no='$TRADE_NO';")" == "paid" ]] \
    && ok "[$LABEL] 充订单 = paid ✓" || bad "[$LABEL] 充订单状态不对 ✗"
done

# ==================== 场景 2：套餐购买（两条：renew / new）==================
step "【套餐购买】两条都要测：renew（续期）+ new（首次购买）"
VAR_ROW="$(sqlq "select id||'|'||price||'|'||duration_days||'|'||name from plan_variants where enabled=1 order by id limit 1;")"
VARIANT_ID="${VAR_ROW%%|*}"; REST="${VAR_ROW#*|}"; VAR_PRICE="${REST%%|*}"; REST="${REST#*|}"; VAR_DAYS="${REST%%|*}"; VAR_NAME="${REST#*|}"
if [[ -z "$VARIANT_ID" || -z "$VAR_PRICE" ]]; then
  bad "找不到可用套餐变体 ✗（plan_variants 为空）"
else
  note "套餐：id=$VARIANT_ID $VAR_NAME 价格=$(yuan "$VAR_PRICE") 元（$VAR_PRICE 分）时长=${VAR_DAYS} 天"

  buy_plan() { # buy_plan <renew|new> <标签>
    local OTYPE="$1" LABEL="$2"
    local bal_before exp_before order trade url pay amt
    bal_before="$(balance)"
    exp_before="$(sqlq "select coalesce(expires_at,'') from clients where email='$CLIENT_EMAIL';")"
    order="$(capi POST /api/v1/client-auth/orders \
      "{\"variant_id\":$VARIANT_ID,\"order_type\":\"$OTYPE\",\"pay_method\":\"charge_site\",\"charge_site_id\":$SITE_ID}")"
    trade="$(jget "$order" "data.trade_no")"
    url="$(jget "$order" "data.payment_url")"
    if [[ -z "$trade" ]]; then bad "[$LABEL] 套餐下单失败 ✗ $(printf '%s' "$order" | head -c 140)"; return; fi
    ok "[$LABEL] 套餐下单成功 ✓ trade_no=$trade"
    amt="$(sqlq "select amount from charge_orders where trade_no='$trade';")"
    [[ "$amt" == "$VAR_PRICE" ]] && ok "[$LABEL] 订单金额 = 套餐价 $amt 分 ✓" \
      || bad "[$LABEL] 订单金额 = $amt ✗（期望 $VAR_PRICE）"
    [[ "$(sqlq "select order_type from charge_orders where trade_no='$trade';")" == "purchase" ]] \
      && ok "[$LABEL] order_type = purchase ✓" || bad "[$LABEL] order_type 不对 ✗"
    [[ -n "$url" && "$url" == "$MOCK_URL/"* ]] && ok "[$LABEL] payment_url 指向模拟站点 ✓" \
      || bad "[$LABEL] payment_url 不对 ✗ $url"
    pay="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$trade&status=paid")"
    [[ "$(jget "$pay" "panel_code")" == "200" ]] && ok "[$LABEL] 套餐回调被受理 ✓" \
      || { bad "[$LABEL] 套餐回调失败 ✗ $pay"; tail -6 "$TMP/mock.log" | sed 's/^/      /'; }
    # —— 套餐真的生效（最硬的证据）——
    [[ "$(sqlq "select coalesce(plan_variant_id,0) from clients where email='$CLIENT_EMAIL';")" == "$VARIANT_ID" ]] \
      && ok "[$LABEL] 客户端套餐 = 该变体 ✓" || bad "[$LABEL] 套餐未生效 ✗"
    [[ "$(sqlq "select coalesce(status,'') from plan_orders where client_id=$CLIENT_ID order by id desc limit 1;")" == "completed" ]] \
      && ok "[$LABEL] 套餐订单 = completed ✓（不是 pending）" || bad "[$LABEL] 套餐订单状态不对 ✗"
    [[ "$(sqlq "select status from charge_orders where trade_no='$trade';")" == "paid" ]] \
      && ok "[$LABEL] 充订单 = paid ✓" || bad "[$LABEL] 充订单状态不对 ✗"
    [[ "$(balance)" == "$bal_before" ]] && ok "[$LABEL] 余额未被改动 ✓（$bal_before）" \
      || bad "[$LABEL] 余额被改动了 ✗ $bal_before → $(balance)"
    local exp_after; exp_after="$(sqlq "select coalesce(expires_at,'') from clients where email='$CLIENT_EMAIL';")"
    note "[$LABEL] 有效期：${exp_before:-<空>} → ${exp_after:-<空>}"
    [[ -n "$exp_after" && "$exp_after" != "$exp_before" ]] \
      && ok "[$LABEL] 有效期已生效/顺延 ✓" || bad "[$LABEL] 有效期没变化 ✗"
    # 非整元套餐价（9.90）能对上，就说明金额单位统一了
    [[ "$VAR_PRICE" != "$(($VAR_PRICE / 100 * 100))" ]] \
      && note "[$LABEL] 这条是非整元价（$(yuan "$VAR_PRICE") 元）→ 金额单位必须按「分」才可能对上 ✓"
  }

  buy_plan renew "续期"
  # 首次购买（new）：清空客户当前套餐 —— 纯测试数据准备，制造"没有在效套餐"的前置状态
  sqlq "update clients set plan_id=0, plan_variant_id=0, expires_at=NULL where email='$CLIENT_EMAIL';" \
    && note "已清空该测试客户端的当前套餐（为 new 用例准备 ✓）"
  buy_plan new "首次购买"
fi

# ==================== 场景 3：订单过期 =======================================
step "【订单过期】站点侧不再收款；但已经收到的款必须入账（正常交易逻辑）"
note "过期 = ①面板把订单标注 expired（门户显示已过期）②站点侧停止收款 —— 拦在「付款前」"
note "但钱要是已经收了（回调照样来了）→ 必须入账：绝不能「用户付了款还失败」（丢单比过期严重）"
E_ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
E_TRADE="$(jget "$E_ORDER" "data.trade_no")"
if [[ -z "$E_TRADE" ]]; then
  bad "过期用例下单失败 ✗ $E_ORDER"
else
  E_EXP="$(sqlq "select coalesce(expires_at,'') from charge_orders where trade_no='$E_TRADE';")"
  [[ -n "$E_EXP" ]] && ok "订单带上了过期时间 ✓ expires_at=$E_EXP" || bad "订单没有 expires_at ✗"
  sqlq "update charge_orders set expires_at = datetime('now','-1 hour') where trade_no='$E_TRADE';" \
    && ok "已把该订单置为过期（测试数据准备 ✓）" || bad "置过期失败 ✗"
  # ① 站点侧：收款窗口到点 → 不再接收付款（拦在付款前 ✓）
  curl -s -X POST "$MOCK_URL/expire?trade_no=$E_TRADE" >/dev/null
  REFUSE="$(curl -s -X POST "$MOCK_URL/pay-api?trade_no=$E_TRADE&status=paid")"
  printf '%s' "$REFUSE" | grep -q "不再接收付款" \
    && ok "站点侧已停止收款 ✓：$(jget "$REFUSE" "error" | head -c 90)" \
    || bad "站点侧竟然还接受付款 ✗ $REFUSE"
  # ② 面板侧：把订单也置为过期（模拟时间已经走过 TTL ✓）
  sqlq "update charge_orders set expires_at = datetime('now','-1 hour') where trade_no='$E_TRADE';"
  sleep 0.3 # 让外部 sqlite3 进程彻底释放写锁（SQLite 是单文件库）
  E_BAL0="$(balance)"
  # ③ 兜底：**万一钱真的收了**（直接投递一份完全合规的回调）→ 必须入账 ✓
  #
  # ⚠ SQLite 特有：WAL 下"读→写升级"会**立刻**返回 SQLITE_BUSY（busy_timeout 不等），
  # 只要此刻有别的写者（面板自家定时任务 / 外部 sqlite3）就会偶发失败。
  # 真实网关收到非 200 会**自动重试**，面板自身也会 enqueueCallbackRetry —— 这里照做（最多 3 次 ✓），
  # 而不是把断言放宽（那样会掩盖真正的失败）。
  E_CODE=""
  E_NONCE="latefix$(date +%s%N)" # ⚠ 重试**复用同一个 nonce**（真实网关就是这样重试的 ✓）
  for attempt in 1 2 3 4 5 6 7 8; do
    # 这条同时验证两件事 ✓：
    #  ① 瞬时失败（SQLite 写锁）后重投能成功 ✓；
    #  ② 重投用的是**同一个 nonce** ✓ —— 面板会把它识别为"同 nonce + 同载荷 = 网关重试"并幂等放行 ✓
    #     （旧实现会判 replay ✗ → 这笔钱永远入不了账 ✗）。
    E_URL="$(make_callback --trade-no "$E_TRADE" --site-id "$SITE_ID" --secret "$SITE_SECRET" \
      --amount "$AMOUNT_FEN" --nonce "$E_NONCE")"
    E_CODE="$(curl -s -o "$TMP/e.out" -w '%{http_code}' -X POST "$E_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
    [[ "$E_CODE" == "200" ]] && break
    if grep -q "database is locked" "$TMP/e.out"; then
      # 放宽到 ~16 秒：本测试自己创建/删除测试客户端会触发票据同步扇出（多节点 + 重启内核），
      # 那个写洪峰可能持续十几秒；真实网关的重试窗口远大于此，这里只是照实模拟。
      info "第 $attempt 次遇到瞬时写锁 → 同 nonce 重投 ✓（网关重试语义）"
      sleep 2
      continue
    fi
    break
  done
  [[ "$E_CODE" == "200" ]] && ok "已收到的款照常入账 → HTTP 200 ✓（不能让用户付款了还失败 ✓）" \
    || bad "已付款的回调被拒 ✗ HTTP $E_CODE $(head -c 140 "$TMP/e.out")"
  E_ST="$(sqlq "select status from charge_orders where trade_no='$E_TRADE';")"
  [[ "$E_ST" == "paid" ]] && ok "订单最终 = paid ✓（过期 → 已付款，状态正确落地 ✓）" \
    || bad "订单状态 = $E_ST ✗（期望 paid ✓）"
  E_DELTA=$(( $(balance) - E_BAL0 ))
  [[ "$E_DELTA" == "$AMOUNT_FEN" ]] && ok "余额 +$E_DELTA 分 ✓（一分不少 ✓）" \
    || bad "入账金额不对 ✗ got=$E_DELTA want=$AMOUNT_FEN"
  # ④ 幂等：重复投递同一回调 → 不重复入账 ✓
  curl -s -o /dev/null -X POST "$E_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json"
  [[ "$(( $(balance) - E_BAL0 ))" == "$AMOUNT_FEN" ]] \
    && ok "重复投递未重复入账 ✓（幂等 ✓）" || bad "重复投递导致重复入账 ✗"
  # 过期订单 + 金额不符 → 必须被拒 ✓（"过期"不是放水，数据仍要严格对上 ✓）
  E2_ORDER="$(capi POST /api/v1/client-auth/charge/create "{\"site_id\":$SITE_ID,\"amount\":$AMOUNT_FEN}")"
  E2_TRADE="$(jget "$E2_ORDER" "data.trade_no")"
  if [[ -n "$E2_TRADE" ]]; then
    sqlq "update charge_orders set expires_at = datetime('now','-1 hour') where trade_no='$E2_TRADE';"
    W_URL="$(make_callback --trade-no "$E2_TRADE" --site-id "$SITE_ID" --secret "$SITE_SECRET" --amount "$((AMOUNT_FEN + 1))")"
    W_CODE="$(curl -s -o "$TMP/w.out" -w '%{http_code}' -X POST "$W_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
    [[ "$W_CODE" == "400" ]] && ok "过期订单 + 差 1 分 → HTTP 400 ✓（金额必须严格对上）" \
      || bad "过期订单的金额不符回调没被拒 ✗ HTTP $W_CODE $(head -c 140 "$TMP/w.out")"
  fi
fi

# ==================== 场景 4：反向用例 ======================================
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
  neg "错误签名"             400 --amount "$AMOUNT_FEN" --break-sign
  neg "金额不符（差 1 分）"    400 --amount "$((AMOUNT_FEN + 1))"
  neg "金额用错单位（元当分）" 400 --amount "$AMOUNT_YUAN"
  neg "时间过期（±300s 外）"  400 --amount "$AMOUNT_FEN" --ts-offset -3600
  RP_NONCE="retry$(date +%s%N)"
  neg "正确回调（给重试用）"            200 --amount "$AMOUNT_FEN" --nonce "$RP_NONCE"
  # ⚠ 同 nonce + **同订单号** = 网关重试 → 必须幂等放行（旧实现判 replay ✗ → 会丢单 ✗）
  #   真实网关重试既可能"逐字节重发"，也可能"重新构造请求"（时间戳变、nonce 与订单不变），
  #   所以判据是"同一笔订单"，而不是"载荷逐字节相同"。
  NEG_BAL="$(balance)"
  neg "同 nonce + 同订单号（网关重试）"  200 --amount "$AMOUNT_FEN" --nonce "$RP_NONCE"
  [[ "$(balance)" == "$NEG_BAL" ]] && ok "重试未重复入账 ✓（幂等 ✓ 余额仍 $NEG_BAL）" \
    || bad "重试导致重复入账 ✗ $NEG_BAL → $(balance)"
  # ⚠ 同 nonce + **不同订单号** = 真重放 → 必须拒绝 ✓（用场景 1 那笔订单的 trade_no ✓）
  if [[ -n "${TRADE_NO:-}" ]]; then
    RP_URL="$(make_callback --trade-no "$TRADE_NO" --site-id "$SITE_ID" --secret "$SITE_SECRET" --amount "$AMOUNT_FEN" --nonce "$RP_NONCE")"
    RP_CODE="$(curl -s -o "$TMP/rp.out" -w '%{http_code}' -X POST "$RP_URL" -H 'Content-Type: application/json' --data-binary @"$TMP/cb.json")"
    [[ "$RP_CODE" == "400" ]] && ok "同 nonce + 不同订单号（真重放） → HTTP 400 ✓" \
      || bad "跨订单的 nonce 重放没被拒 ✗ HTTP $RP_CODE $(head -c 140 "$TMP/rp.out")"
  fi
else
  bad "反向用例的订单创建失败 ✗ $NEG_ORDER"
fi

step "清理"
ok "测试站点与测试客户端由退出钩子统一删除 ✓（site=$SITE_ID client=$CLIENT_ID）"
if [[ -z "$KEEP" ]]; then info "模拟站点已停止 ✓（KEEP=1 可保留）"; else info "模拟站点保留在 $MOCK_URL ✓（收银台 $MOCK_URL/orders）"; fi

printf '\n\033[1m========== 汇总：通过 %d ✓ / 失败 %d ✗ ==========\033[0m\n' "$PASS" "$FAIL"
[[ "$FAIL" == "0" ]] && exit 0 || exit 1
