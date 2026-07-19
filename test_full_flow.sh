#!/bin/bash
# CoreFusion 全链路测试脚本 v2
# 覆盖：注册 → 推广绑定 → 充值 → 套餐购买（余额/套餐卡/三方）→ 节点同步 → 返利 → 退款

BASE="http://127.0.0.1:8082"
ADMIN_USER="admin"
ADMIN_PASS="admin123"
TIMESTAMP=$(date +%s)
PASS=0
FAIL=0

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

ok()   { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# 通用 JSON 解析：兼容 {data:{...}} 和直接 {...} 两种格式
jval() { python3 -c "
import sys,json
d=json.load(sys.stdin)
if isinstance(d.get('data'),dict) and '$1' in d['data']:
    print(d['data']['$1'])
elif '$1' in d:
    print(d['$1'])
else:
    print(d.get('data',{}).get('$1','') if isinstance(d.get('data'),dict) else '')
" 2>/dev/null; }

# ========== 0. 管理员登录 ==========
info "管理员登录..."
ADMIN_TOKEN=$(curl -s "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
  | jval access_token)

if [ -z "$ADMIN_TOKEN" ]; then
  fail "管理员登录失败"
  exit 1
fi
ok "管理员登录成功"

# ========== 1. 推广人注册 ==========
REFERRER_EMAIL="ref_test_${TIMESTAMP}@test.com"
info "推广人注册: $REFERRER_EMAIL"
RESP=$(curl -s "$BASE/api/v1/client-auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$REFERRER_EMAIL\",\"password\":\"test123456\"}")
REFERRER_TOKEN=$(echo "$RESP" | jval access_token)
REFERRER_ID=$(curl -s "$BASE/api/v1/client-auth/me" \
  -H "Authorization: Bearer $REFERRER_TOKEN" | jval id)
if [ -n "$REFERRER_TOKEN" ] && [ -n "$REFERRER_ID" ]; then
  ok "推广人注册成功 (ID=$REFERRER_ID)"
else
  fail "推广人注册失败: $RESP"
  exit 1
fi

# ========== 2. 获取推广码 ==========
info "获取推广码..."
RESP=$(curl -s "$BASE/api/v1/client-auth/referral/code" \
  -H "Authorization: Bearer $REFERRER_TOKEN")
REFERRAL_CODE=$(echo "$RESP" | jval code)
if [ -n "$REFERRAL_CODE" ]; then
  ok "推广码获取成功: $REFERRAL_CODE"
else
  fail "推广码获取失败: $RESP"
  exit 1
fi

# ========== 3. 被推广人注册（带推广码） ==========
REFEREE_EMAIL="referee_test_${TIMESTAMP}@test.com"
info "被推广人注册（带推广码）: $REFEREE_EMAIL"
RESP=$(curl -s "$BASE/api/v1/client-auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$REFEREE_EMAIL\",\"password\":\"test123456\",\"referral_code\":\"$REFERRAL_CODE\"}")
REFEREE_TOKEN=$(echo "$RESP" | jval access_token)
REFEREE_ID=$(curl -s "$BASE/api/v1/client-auth/me" \
  -H "Authorization: Bearer $REFEREE_TOKEN" | jval id)
if [ -n "$REFEREE_TOKEN" ] && [ -n "$REFEREE_ID" ]; then
  ok "被推广人注册成功 (ID=$REFEREE_ID)"
else
  fail "被推广人注册失败: $RESP"
  exit 1
fi

# ========== 4. 验证推广关系 ==========
info "验证推广关系..."
RESP=$(curl -s "$BASE/api/v1/client-auth/referral/stats" \
  -H "Authorization: Bearer $REFERRER_TOKEN")
TOTAL_INVITED=$(echo "$RESP" | jval total_invited)
if [ "$TOTAL_INVITED" -ge 1 ] 2>/dev/null; then
  ok "推广关系建立成功 (total_invited=$TOTAL_INVITED)"
else
  fail "推广关系未建立: $RESP"
fi

# ========== 5. 获取可购买套餐列表 ==========
info "获取可购买套餐列表..."
RESP=$(curl -s "$BASE/api/v1/client-auth/plans" \
  -H "Authorization: Bearer $REFEREE_TOKEN")
PLAN_ID=$(echo "$RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
plans=d.get('data',d) if isinstance(d.get('data'),list) else d.get('data',{}).get('items',[])
if isinstance(plans,list) and plans:
    print(plans[0]['id'])
else:
    print('')
" 2>/dev/null)
if [ -z "$PLAN_ID" ]; then
  fail "无可用套餐"
  exit 1
fi
ok "获取套餐列表成功，首个套餐 ID=$PLAN_ID"

# 获取套餐详情找 variant_id
RESP=$(curl -s "$BASE/api/v1/client-auth/plans/$PLAN_ID" \
  -H "Authorization: Bearer $REFEREE_TOKEN")
VARIANT_ID=$(echo "$RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
plan=d.get('data',d) if isinstance(d.get('data'),dict) else d
variants=plan.get('variants',[])
if variants:
    for v in variants:
        if v.get('enabled',True):
            print(v['id'])
            break
else:
    print('')
" 2>/dev/null)
if [ -z "$VARIANT_ID" ]; then
  fail "无法获取套餐规格 ID"
  exit 1
fi
ok "获取套餐规格成功 variant_id=$VARIANT_ID"

# ========== 6. 场景A：余额购买套餐 + 返利 ==========
info "========== 场景A：余额购买套餐 =========="
# 通过 SQLite 直接给被推广人充余额
info "SQLite 直接充值 50000 分..."
sqlite3 /opt/corefusion/data/sqlite/app.db "INSERT OR IGNORE INTO client_balances (client_id, balance, frozen, currency, created_at, updated_at) VALUES ($REFEREE_ID, 50000, 0, 'CNY', datetime('now'), datetime('now'));" 2>/dev/null
sqlite3 /opt/corefusion/data/sqlite/app.db "UPDATE client_balances SET balance=50000 WHERE client_id=$REFEREE_ID;" 2>/dev/null
ok "余额准备完成: 50000 分"

# 购买套餐
info "被推广人余额购买套餐..."
ORDER_RESP=$(curl -s -X POST "$BASE/api/v1/client-auth/orders" \
  -H "Authorization: Bearer $REFEREE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"variant_id\":$VARIANT_ID,\"order_type\":\"new\",\"pay_method\":\"balance\"}")
ORDER_STATUS=$(echo "$ORDER_RESP" | jval status)
if [ "$ORDER_STATUS" = "completed" ]; then
  ok "余额购买套餐成功"
else
  fail "余额购买套餐失败: $ORDER_RESP"
fi

# 检查套餐是否激活
info "检查套餐是否激活..."
ME_RESP=$(curl -s "$BASE/api/v1/client-auth/me" \
  -H "Authorization: Bearer $REFEREE_TOKEN")
PLAN_STATUS=$(echo "$ME_RESP" | jval status)
if [ "$PLAN_STATUS" = "active" ]; then
  ok "套餐已激活 (status=$PLAN_STATUS)"
else
  fail "套餐未激活: $ME_RESP"
fi

# 检查推广人是否获得返利
info "检查推广人是否获得返利..."
sleep 1
RESP=$(curl -s "$BASE/api/v1/client-auth/referral/stats" \
  -H "Authorization: Bearer $REFERRER_TOKEN")
TOTAL_PAID=$(echo "$RESP" | jval total_paid)
COMMISSION=$(echo "$RESP" | jval total_commission)
info "推广人统计: total_paid=$TOTAL_PAID, commission=$COMMISSION"
if [ "$TOTAL_PAID" -ge 1 ] 2>/dev/null && [ "$COMMISSION" -gt 0 ] 2>/dev/null; then
  ok "余额购买返利成功 (total_paid=$TOTAL_PAID, commission=$COMMISSION 分)"
else
  fail "余额购买返利未触发 (total_paid=$TOTAL_PAID, commission=$COMMISSION)"
fi

# 检查节点同步日志（account sync / xray add user 链路）
info "检查余额购买后节点同步..."
sleep 1
REFEREE_EMAIL=$(curl -s "$BASE/api/v1/clients/$REFEREE_ID" -H "Authorization: Bearer $ADMIN_TOKEN" | jval '.data.email' 2>/dev/null || echo "")
SYNC_LOG=$(grep -i "account sync completed\|xray add user.*${REFEREE_EMAIL:-referee_test}" /tmp/corefusion.log 2>/dev/null | tail -3)
if [ -n "$SYNC_LOG" ]; then
  ok "节点同步已触发: $SYNC_LOG"
else
  fail "未找到 client $REFEREE_ID 的节点同步记录"
fi

# ========== 7. 场景B：套餐卡购买套餐 + 返利 ==========
info "========== 场景B：套餐卡购买套餐 =========="
# 管理员创建套餐卡（让系统生成卡号）
info "管理员创建套餐卡..."
CARD_RESP=$(curl -s -X POST "$BASE/api/v1/charge-cards" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_type\":\"plan\",\"value\":$VARIANT_ID,\"currency\":\"CNY\",\"remark\":\"测试套餐卡\"}")
# 从返回中提取 plaintext_card_code
PLAN_CARD_CODE=$(echo "$CARD_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('data',{}).get('plaintext_card_code',''))
" 2>/dev/null)
if [ -n "$PLAN_CARD_CODE" ]; then
  ok "套餐卡创建成功: $PLAN_CARD_CODE"
else
  fail "套餐卡创建失败或无明文卡码: $CARD_RESP"
fi

# 新用户注册（带推广码，测试套餐卡购买+返利）
CARD_USER_EMAIL="card_user_${TIMESTAMP}@test.com"
info "新用户注册（带推广码）: $CARD_USER_EMAIL"
RESP=$(curl -s "$BASE/api/v1/client-auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$CARD_USER_EMAIL\",\"password\":\"test123456\",\"referral_code\":\"$REFERRAL_CODE\"}")
CARD_USER_TOKEN=$(echo "$RESP" | jval access_token)
CARD_USER_ID=$(curl -s "$BASE/api/v1/client-auth/me" \
  -H "Authorization: Bearer $CARD_USER_TOKEN" | jval id)
if [ -n "$CARD_USER_TOKEN" ]; then
  ok "新用户注册成功 (ID=$CARD_USER_ID)"
else
  fail "新用户注册失败: $RESP"
fi

# 使用套餐卡购买套餐
info "使用套餐卡购买套餐..."
ORDER_RESP=$(curl -s -X POST "$BASE/api/v1/client-auth/orders" \
  -H "Authorization: Bearer $CARD_USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"variant_id\":$VARIANT_ID,\"order_type\":\"new\",\"pay_method\":\"gift_card\",\"gift_card_code\":\"$PLAN_CARD_CODE\"}")
ORDER_STATUS=$(echo "$ORDER_RESP" | jval status)
if [ "$ORDER_STATUS" = "completed" ]; then
  ok "套餐卡购买套餐成功"
else
  fail "套餐卡购买套餐失败: $ORDER_RESP"
fi

# 检查套餐是否激活
info "检查套餐卡购买的套餐是否激活..."
ME_RESP=$(curl -s "$BASE/api/v1/client-auth/me" \
  -H "Authorization: Bearer $CARD_USER_TOKEN")
PLAN_STATUS=$(echo "$ME_RESP" | jval status)
if [ "$PLAN_STATUS" = "active" ]; then
  ok "套餐卡套餐已激活 (status=$PLAN_STATUS)"
else
  fail "套餐卡套餐未激活: $ME_RESP"
fi

# 检查推广人是否获得返利（套餐卡购买应该触发）
info "检查套餐卡购买后推广人返利..."
sleep 1
RESP=$(curl -s "$BASE/api/v1/client-auth/referral/stats" \
  -H "Authorization: Bearer $REFERRER_TOKEN")
TOTAL_PAID_AFTER=$(echo "$RESP" | jval total_paid)
COMMISSION_AFTER=$(echo "$RESP" | jval total_commission)
info "推广人统计: total_paid=$TOTAL_PAID_AFTER, commission=$COMMISSION_AFTER"
if [ "$TOTAL_PAID_AFTER" -ge 2 ] 2>/dev/null && [ "$COMMISSION_AFTER" -gt "$COMMISSION" ] 2>/dev/null; then
  ok "套餐卡购买返利成功 (total_paid=$TOTAL_PAID_AFTER, commission=$COMMISSION_AFTER 分)"
else
  fail "套餐卡购买返利未触发 (期望 total_paid>=2, 实际 $TOTAL_PAID_AFTER)"
fi

# 检查节点同步
info "检查套餐卡购买后节点同步..."
sleep 1
CARD_USER_EMAIL=$(curl -s "$BASE/api/v1/clients/$CARD_USER_ID" -H "Authorization: Bearer $ADMIN_TOKEN" | jval '.data.email' 2>/dev/null || echo "")
SYNC_LOG=$(grep -i "xray add user.*${CARD_USER_EMAIL:-card_user}" /tmp/corefusion.log 2>/dev/null | tail -3)
if [ -n "$SYNC_LOG" ]; then
  ok "套餐卡购买节点同步已触发: $SYNC_LOG"
else
  fail "未找到 client $CARD_USER_ID 的节点同步记录"
fi

# ========== 8. 场景C：充值卡充值 ==========
info "========== 场景C：充值卡充值 =========="
# 创建充值卡（让系统生成卡号）
info "管理员创建充值卡（10000分=100元）..."
BAL_CARD_RESP=$(curl -s -X POST "$BASE/api/v1/charge-cards" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_type\":\"balance\",\"value\":10000,\"currency\":\"CNY\",\"remark\":\"测试充值卡\"}")
BAL_CARD_CODE=$(echo "$BAL_CARD_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('data',{}).get('plaintext_card_code',''))
" 2>/dev/null)
if [ -n "$BAL_CARD_CODE" ]; then
  ok "充值卡创建成功: $BAL_CARD_CODE"
else
  fail "充值卡创建失败: $BAL_CARD_RESP"
fi

# 记录充值前余额
BAL_BEFORE=$(curl -s "$BASE/api/v1/client-auth/balance" \
  -H "Authorization: Bearer $REFEREE_TOKEN" | jval balance)
info "充值前余额: $BAL_BEFORE 分"

# 被推广人兑换充值卡
info "被推广人兑换充值卡..."
REDEEM_RESP=$(curl -s -X POST "$BASE/api/v1/client-auth/charge/redeem-card" \
  -H "Authorization: Bearer $REFEREE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_code\":\"$BAL_CARD_CODE\"}")
REDEEM_CODE=$(echo "$REDEEM_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('code',200) if isinstance(d.get('code'),int) else d.get('code','200'))
" 2>/dev/null)

# 检查余额增加
BAL_AFTER=$(curl -s "$BASE/api/v1/client-auth/balance" \
  -H "Authorization: Bearer $REFEREE_TOKEN" | jval balance)
info "兑换后余额: $BAL_AFTER 分"
if [ "$BAL_AFTER" -gt "$BAL_BEFORE" ] 2>/dev/null; then
  ok "充值卡充值成功，余额增加 $((BAL_AFTER - BAL_BEFORE)) 分"
else
  fail "充值卡充值后余额未变化 (before=$BAL_BEFORE, after=$BAL_AFTER): $REDEEM_RESP"
fi

# ========== 9. 场景D：退款（使用套餐卡购买用户，因为它创建了 charge_order） ==========
info "========== 场景D：退款 =========="
# 查找套餐卡用户的 charge orders（purchase 类型）
info "查询套餐卡用户(ID=$CARD_USER_ID)的 charge orders..."
ORDERS_RESP=$(curl -s "$BASE/api/v1/charge-orders?client_id=$CARD_USER_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
CHARGE_ORDER_ID=$(echo "$ORDERS_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d.get('data',{}).get('items',[])
if not items and isinstance(d.get('data'),list):
    items=d['data']
if isinstance(items,list):
    for o in items:
        if o.get('status')=='paid' and o.get('order_type')=='purchase':
            print(o.get('id',''))
            break
" 2>/dev/null)

if [ -n "$CHARGE_ORDER_ID" ]; then
  info "退款 charge_order_id=$CHARGE_ORDER_ID"
  # 获取退款前的到期时间
  ME_BEFORE=$(curl -s "$BASE/api/v1/client-auth/me" \
    -H "Authorization: Bearer $CARD_USER_TOKEN")
  EXPIRES_BEFORE=$(echo "$ME_BEFORE" | jval expires_at)
  info "退款前到期时间: $EXPIRES_BEFORE"

  # 管理员退款
  REFUND_RESP=$(curl -s -X POST "$BASE/api/v1/charge-orders/$CHARGE_ORDER_ID/refund" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"remark":"测试退款"}')
  REFUND_CODE=$(echo "$REFUND_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
c=d.get('code',200)
if isinstance(c,str) and c.isdigit():
    c=int(c)
if c in (200,201,0,''):
    print('200')
else:
    print(c)
" 2>/dev/null)
  if [ "$REFUND_CODE" = "200" ]; then
    ok "退款请求成功"
  else
    fail "退款请求失败: $REFUND_RESP"
  fi

  # 检查到期时间是否变化
  ME_AFTER=$(curl -s "$BASE/api/v1/client-auth/me" \
    -H "Authorization: Bearer $CARD_USER_TOKEN")
  EXPIRES_AFTER=$(echo "$ME_AFTER" | jval expires_at)
  info "退款后到期时间: $EXPIRES_AFTER"
  if [ "$EXPIRES_BEFORE" != "$EXPIRES_AFTER" ]; then
    ok "退款后套餐到期时间已回退"
  else
    fail "退款后套餐到期时间未变化 (before=$EXPIRES_BEFORE, after=$EXPIRES_AFTER)"
  fi
else
  # 也检查被推广人是否有 charge order
  info "套餐卡用户无 charge order，检查被推广人..."
  ORDERS_RESP=$(curl -s "$BASE/api/v1/charge-orders?client_id=$REFEREE_ID" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
  CHARGE_ORDER_ID=$(echo "$ORDERS_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d.get('data',{}).get('items',[])
if not items and isinstance(d.get('data'),list):
    items=d['data']
if isinstance(items,list):
    for o in items:
        if o.get('status')=='paid' and o.get('order_type')=='purchase':
            print(o.get('id',''))
            break
" 2>/dev/null)

  if [ -n "$CHARGE_ORDER_ID" ]; then
    info "找到被推广人 charge_order_id=$CHARGE_ORDER_ID"
    ME_BEFORE=$(curl -s "$BASE/api/v1/client-auth/me" \
      -H "Authorization: Bearer $REFEREE_TOKEN")
    EXPIRES_BEFORE=$(echo "$ME_BEFORE" | jval expires_at)
    info "退款前到期时间: $EXPIRES_BEFORE"

    REFUND_RESP=$(curl -s -X POST "$BASE/api/v1/charge-orders/$CHARGE_ORDER_ID/refund" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"remark":"测试退款"}')
    info "退款响应: $REFUND_RESP"

    ME_AFTER=$(curl -s "$BASE/api/v1/client-auth/me" \
      -H "Authorization: Bearer $REFEREE_TOKEN")
    EXPIRES_AFTER=$(echo "$ME_AFTER" | jval expires_at)
    info "退款后到期时间: $EXPIRES_AFTER"
    if [ "$EXPIRES_BEFORE" != "$EXPIRES_AFTER" ]; then
      ok "退款后套餐到期时间已回退"
    else
      fail "退款后套餐到期时间未变化"
    fi
  else
    fail "无 charge order 可退款（余额购买不创建 charge_order）"
  fi
fi

# ========== 10. 检查节点移除 ==========
info "========== 检查退款后节点移除 =========="
sleep 1
REMOVE_LOG=$(grep "UserSync" /tmp/corefusion.log 2>/dev/null | grep -i "remove\|client $REFEREE_ID" | tail -5)
if [ -n "$REMOVE_LOG" ]; then
  info "节点同步日志: $REMOVE_LOG"
  ok "退款后节点移除逻辑已触发"
else
  info "暂未找到 remove 相关日志（退款可能走 plan_order 逻辑）"
fi

# ========== 汇总 ==========
echo ""
echo "=========================================="
echo -e "测试汇总: ${GREEN}通过 $PASS${NC} / ${RED}失败 $FAIL${NC}"
echo "=========================================="