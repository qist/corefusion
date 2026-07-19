#!/bin/bash
# 测试：过期后重新购买套餐，验证账号同步是否触发
BASE="http://127.0.0.1:8082/api/v1"
ADMIN_TOKEN=""
TEST_USER_EMAIL="expire_rebuy_$(date +%s)@test.com"
TEST_USER_ID=""
PLAN_VARIANT_ID=2

ok() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1"; }
info() { echo "[INFO] $1"; }

jval() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)" 2>/dev/null; }

# ========== 0. 管理员登录 ==========
info "管理员登录..."
ADMIN_RESP=$(curl -s -X POST "$BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jval "['data']['access_token']")
if [ -z "$ADMIN_TOKEN" ]; then
  fail "管理员登录失败"
  exit 1
fi
ok "管理员登录成功"

# ========== 1. 注册新用户 ==========
info "注册新用户: $TEST_USER_EMAIL"
REG_RESP=$(curl -s -X POST "$BASE/client-auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_USER_EMAIL\",\"password\":\"test123456\"}")
TEST_USER_ID=$(echo "$REG_RESP" | jval "['data']['client']['id']")
if [ -z "$TEST_USER_ID" ]; then
  fail "注册失败: $REG_RESP"
  exit 1
fi
ok "注册成功 (ID=$TEST_USER_ID)"

# 用户登录获取 token
login_user() {
  local RESP=$(curl -s -X POST "$BASE/client-auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$TEST_USER_EMAIL\",\"password\":\"test123456\"}")
  echo "$RESP" | jval "['data']['access_token']"
}

USER_TOKEN=$(login_user)

# ========== 2. 创建充值卡并充值余额 ==========
info "创建充值卡（50000分=500元）..."
CARD_RESP=$(curl -s -X POST "$BASE/charge-cards" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"card_type":"balance","value":50000,"currency":"CNY","remark":"测试充值"}')
CARD_CODE=$(echo "$CARD_RESP" | jval "['data']['plaintext_card_code']")
if [ -z "$CARD_CODE" ]; then
  fail "充值卡创建失败: $CARD_RESP"
  exit 1
fi
ok "充值卡创建成功: $CARD_CODE"

info "用户兑换充值卡..."
REDEEM_RESP=$(curl -s -X POST "$BASE/client-auth/charge/redeem-card" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_code\":\"$CARD_CODE\"}")
ok "充值卡兑换完成"

# ========== 3. 余额购买套餐（首次激活）==========
info "余额购买套餐（首次）..."
ORDER_RESP=$(curl -s -X POST "$BASE/client-auth/orders" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"variant_id\":$PLAN_VARIANT_ID,\"pay_method\":\"balance\",\"order_type\":\"new\"}")
ORDER_STATUS=$(echo "$ORDER_RESP" | jval "['data']['status']")
if [ "$ORDER_STATUS" != "completed" ]; then
  fail "首次购买失败: $ORDER_RESP"
  exit 1
fi
ok "首次购买成功"

# 检查套餐状态
sleep 2
CLIENT_INFO=$(curl -s "$BASE/clients/$TEST_USER_ID" -H "Authorization: Bearer $ADMIN_TOKEN")
CLIENT_STATUS=$(echo "$CLIENT_INFO" | jval "['data']['status']")
EXPIRES_AT=$(echo "$CLIENT_INFO" | jval "['data']['expires_at']")
ok "套餐状态: $CLIENT_STATUS, 到期时间: $EXPIRES_AT"

# 检查账号同步日志
SYNC_LOG=$(grep "xray add user.*${TEST_USER_EMAIL}" /tmp/corefusion.log 2>/dev/null | tail -3)
if [ -n "$SYNC_LOG" ]; then
  ok "首次购买账号同步成功"
else
  fail "首次购买未找到账号同步日志"
fi

# ========== 4. 手动将客户端标记为过期 ==========
info "手动将客户端标记为过期..."
sqlite3 /opt/corefusion/data/sqlite/app.db \
  "UPDATE clients SET status='expired', expires_at='2020-01-01 00:00:00+00:00' WHERE id=$TEST_USER_ID;" 2>/dev/null
sleep 1
CLIENT_STATUS=$(curl -s "$BASE/clients/$TEST_USER_ID" -H "Authorization: Bearer $ADMIN_TOKEN" | jval "['data']['status']")
ok "客户端已标记为过期 (status=$CLIENT_STATUS)"

# ========== 5. 过期后重新购买套餐 ==========
# 记录当前时间作为检查点
MARKER_TIME=$(date +"%H:%M:%S")
info "过期重新购买检查点时间: $MARKER_TIME"

# 再创建一张充值卡充值
info "创建第二张充值卡..."
CARD_RESP2=$(curl -s -X POST "$BASE/charge-cards" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"card_type":"balance","value":50000,"currency":"CNY","remark":"测试充值2"}')
CARD_CODE2=$(echo "$CARD_RESP2" | jval "['data']['plaintext_card_code']")

USER_TOKEN=$(login_user)
info "兑换第二张充值卡..."
curl -s -X POST "$BASE/client-auth/charge/redeem-card" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_code\":\"$CARD_CODE2\"}" > /dev/null

info "过期后重新购买套餐（order_type=new）..."
ORDER_RESP=$(curl -s -X POST "$BASE/client-auth/orders" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"variant_id\":$PLAN_VARIANT_ID,\"pay_method\":\"balance\",\"order_type\":\"new\"}")
ORDER_STATUS=$(echo "$ORDER_RESP" | jval "['data']['status']")
if [ "$ORDER_STATUS" != "completed" ]; then
  fail "过期后重新购买失败: $ORDER_RESP"
  exit 1
fi
ok "过期后重新购买成功"

# 检查套餐状态
sleep 2
CLIENT_INFO=$(curl -s "$BASE/clients/$TEST_USER_ID" -H "Authorization: Bearer $ADMIN_TOKEN")
CLIENT_STATUS=$(echo "$CLIENT_INFO" | jval "['data']['status']")
EXPIRES_AT=$(echo "$CLIENT_INFO" | jval "['data']['expires_at']")
ok "重新购买后状态: $CLIENT_STATUS, 到期时间: $EXPIRES_AT"

# ========== 6. 检查账号同步是否触发 ==========
info "检查过期重新购买后账号同步（检查 $MARKER_TIME 之后的日志）..."
sleep 2
SYNC_LOG=$(awk -v t="$MARKER_TIME" '$0 ~ /[0-9]{2}:[0-9]{2}:[0-9]{2}/ { ts=substr($2,1,8); if (ts >= t) print }' /tmp/corefusion.log | grep -i "account sync completed\|account sync complete" | tail -3)
if [ -n "$SYNC_LOG" ]; then
  ok "过期重新购买后账号同步已触发:"
  echo "$SYNC_LOG" | head -3
else
  fail "过期重新购买后未找到账号同步日志"
fi

# ========== 7. 测试续费场景（非过期）==========
info "测试续费场景（非过期）..."
RENEW_MARKER_TIME=$(date +"%H:%M:%S")
info "续费检查点时间: $RENEW_MARKER_TIME"

# 再充值
CARD_RESP3=$(curl -s -X POST "$BASE/charge-cards" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"card_type":"balance","value":50000,"currency":"CNY","remark":"测试充值3"}')
CARD_CODE3=$(echo "$CARD_RESP3" | jval "['data']['plaintext_card_code']")
USER_TOKEN=$(login_user)
curl -s -X POST "$BASE/client-auth/charge/redeem-card" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"card_code\":\"$CARD_CODE3\"}" > /dev/null

info "续费套餐..."
ORDER_RESP=$(curl -s -X POST "$BASE/client-auth/orders" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"variant_id\":$PLAN_VARIANT_ID,\"pay_method\":\"balance\",\"order_type\":\"renew\"}")
ORDER_STATUS=$(echo "$ORDER_RESP" | jval "['data']['status']")
if [ "$ORDER_STATUS" = "completed" ]; then
  ok "续费成功"
else
  fail "续费失败: $ORDER_RESP"
fi

info "检查续费后账号同步（检查 $RENEW_MARKER_TIME 之后的日志）..."
sleep 2
RENEW_SYNC_LOG=$(awk -v t="$RENEW_MARKER_TIME" '$0 ~ /[0-9]{2}:[0-9]{2}:[0-9]{2}/ { ts=substr($2,1,8); if (ts >= t) print }' /tmp/corefusion.log | grep -i "account sync completed\|account sync complete" | tail -3)
if [ -n "$RENEW_SYNC_LOG" ]; then
  ok "续费后账号同步已触发（幂等检查，已存在账号会跳过）:"
  echo "$RENEW_SYNC_LOG" | head -3
else
  fail "续费后未找到账号同步日志"
fi

echo ""
echo "=========================================="
echo "过期重新购买测试完成"
echo "=========================================="
