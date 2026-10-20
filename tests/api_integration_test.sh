#!/bin/bash
# CoreFusion API Integration Test Suite
# Tests all API endpoints systematically
# Usage: bash tests/api_integration_test.sh [BASE_URL]
set -euo pipefail

BASE="${1:-http://127.0.0.1:8080}"
PASS=0
FAIL=0
SKIP=0
ERRORS=()

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# --- Helpers ---
get_token() {
  curl -s -m 10 "$BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' \
    | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null
}

get_status() {
  local resp="$1"
  # Try to parse as JSON and check for error code
  local result
  result=$(printf '%s' "$resp" | python3 -c "
import sys, json
raw = sys.stdin.read()
try:
    d = json.loads(raw)
    if isinstance(d, list):
        sys.stdout.write('ok')
    elif isinstance(d, dict):
        c = d.get('code', None)
        if c is None or c == 0 or c == '':
            sys.stdout.write('ok')
        else:
            sys.stdout.write(str(c))
    else:
        sys.stdout.write('ok')
except:
    if raw.strip():
        sys.stdout.write('ok')
    else:
        sys.stdout.write('parse_error')
" 2>/dev/null)
  printf '%s' "$result"
}

check() {
  local name="$1"
  local resp="$2"
  local expect="$3"  # "ok" or "error"
  local status
  status=$(get_status "$resp")
  if [ "$expect" = "ok" ] && [ "$status" = "ok" ]; then
    echo -e "  ${GREEN}✓ PASS${NC} $name"
    PASS=$((PASS+1))
  elif [ "$expect" = "error" ] && [ "$status" != "ok" ]; then
    echo -e "  ${GREEN}✓ PASS${NC} $name (expected error: $status)"
    PASS=$((PASS+1))
  else
    echo -e "  ${RED}✗ FAIL${NC} $name (got: $status)"
    FAIL=$((FAIL+1))
    ERRORS+=("$name")
  fi
}

check_http() {
  local name="$1"
  local code="$2"
  local expect="$3"
  if [ "$code" = "$expect" ]; then
    echo -e "  ${GREEN}✓ PASS${NC} $name (HTTP $code)"
    PASS=$((PASS+1))
  else
    echo -e "  ${RED}✗ FAIL${NC} $name (HTTP $code, expected $expect)"
    FAIL=$((FAIL+1))
    ERRORS+=("$name")
  fi
}

req() {
  local method="$1"
  local path="$2"
  local token="$3"
  local data="${4:-}"
  local hdrs=("-s" "-m" "10" "-X" "$method" "$BASE$path")
  [ -n "$token" ] && hdrs+=("-H" "Authorization: Bearer $token")
  [ -n "$data" ] && hdrs+=("-H" "Content-Type: application/json" "-d" "$data")
  curl "${hdrs[@]}" 2>/dev/null
}

req_code() {
  local method="$1"
  local path="$2"
  local token="$3"
  local data="${4:-}"
  local hdrs=("-s" "-m" "10" "-o" "/dev/null" "-w" "%{http_code}" "-X" "$method" "$BASE$path")
  [ -n "$token" ] && hdrs+=("-H" "Authorization: Bearer $token")
  [ -n "$data" ] && hdrs+=("-H" "Content-Type: application/json" "-d" "$data")
  curl "${hdrs[@]}" 2>/dev/null
}

echo "================================================"
echo "  CoreFusion API Integration Test Suite"
echo "  Base URL: $BASE"
echo "================================================"

# --- Get Admin Token ---
echo -e "\n${CYAN}[1] Authentication${NC}"
ADMIN_TOKEN=$(get_token)
if [ -n "$ADMIN_TOKEN" ]; then
  echo -e "  ${GREEN}✓ PASS${NC} Admin login"
  PASS=$((PASS+1))
else
  echo -e "  ${RED}✗ FAIL${NC} Admin login"
  FAIL=$((FAIL+1))
  echo "Cannot continue without admin token"
  exit 1
fi

# Invalid login
RESP=$(curl -s -m 10 "$BASE/api/v1/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"wrong"}')
check "Invalid login rejected" "$RESP" "error"

# Get non-admin token for permission tests
RESP=$(curl -s -m 10 "$BASE/api/v1/auth/login" -H "Content-Type: application/json" -d '{"username":"viewer","password":"viewer123"}' 2>/dev/null)
VIEWER_TOKEN=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('access_token',''))" 2>/dev/null || echo "")

# --- Public Endpoints ---
echo -e "\n${CYAN}[2] Public Endpoints${NC}"
RESP=$(curl -s -m 10 "$BASE/api/v1/public/settings")
check "GET /public/settings" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/public/portal-config")
check "GET /public/portal-config" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/public/articles")
check "GET /public/articles" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/public/subscription-endpoints")
check "GET /public/subscription-endpoints" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/public/client-configs")
check "GET /public/client-configs" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/oauth/providers")
check "GET /client-auth/oauth/providers" "$RESP" "ok"

# --- Client Auth (Public) ---
echo -e "\n${CYAN}[3] Client Auth (Public)${NC}"
RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/register" -H "Content-Type: application/json" -d '{"email":"apitest-'$RANDOM'@test.com","password":"Test123456"}')
check "POST /client-auth/register" "$RESP" "ok"
CLIENT_TOKEN=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null)

# Login with the newly registered user
RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/login" -H "Content-Type: application/json" -d '{"email":"apitest-login@test.com","password":"Test123456"}')
# First register, then login
REGRESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/register" -H "Content-Type: application/json" -d '{"email":"apitest-login@test.com","password":"Test123456"}')
LOGIN_EMAIL=$(echo "$REGRESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('client',{}).get('email','apitest-login@test.com'))" 2>/dev/null || echo 'apitest-login@test.com')
RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/login" -H "Content-Type: application/json" -d '{"email":"'$LOGIN_EMAIL'","password":"Test123456"}')
check "POST /client-auth/login (existing user)" "$RESP" "ok"

RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/login" -H "Content-Type: application/json" -d '{"email":"nonexist@test.com","password":"wrong"}')
check "POST /client-auth/login (invalid credentials)" "$RESP" "error"

# --- Client Auth (Protected) ---
echo -e "\n${CYAN}[4] Client Auth (Protected)${NC}"
if [ -z "$CLIENT_TOKEN" ]; then
  # Register a new client for testing
  RESP=$(curl -s -m 10 "$BASE/api/v1/client-auth/register" -H "Content-Type: application/json" -d '{"email":"apitest2-'$RANDOM'@test.com","password":"Test123456"}')
  CLIENT_TOKEN=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null)
fi

RESP=$(req GET "/api/v1/client-auth/me" "$CLIENT_TOKEN")
check "GET /client-auth/me" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/traffic-history" "$CLIENT_TOKEN")
check "GET /client-auth/traffic-history" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/plans" "$CLIENT_TOKEN")
check "GET /client-auth/plans" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/plans/1" "$CLIENT_TOKEN")
check "GET /client-auth/plans/1" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/traffic-packs" "$CLIENT_TOKEN")
check "GET /client-auth/traffic-packs" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/orders" "$CLIENT_TOKEN")
check "GET /client-auth/orders" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/balance" "$CLIENT_TOKEN")
check "GET /client-auth/balance" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/balance/transactions" "$CLIENT_TOKEN")
check "GET /client-auth/balance/transactions" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/charge/sites" "$CLIENT_TOKEN")
check "GET /client-auth/charge/sites" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/charge/orders" "$CLIENT_TOKEN")
check "GET /client-auth/charge/orders" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/nodes" "$CLIENT_TOKEN")
check "GET /client-auth/nodes" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/gifts" "$CLIENT_TOKEN")
check "GET /client-auth/gifts" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/articles" "$CLIENT_TOKEN")
check "GET /client-auth/articles" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/checktime" "$CLIENT_TOKEN")
check "GET /client-auth/checktime" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/connection-info" "$CLIENT_TOKEN")
check "GET /client-auth/connection-info" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/referral/withdrawals" "$CLIENT_TOKEN")
check "GET /client-auth/referral/withdrawals" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/shared-accounts" "$CLIENT_TOKEN")
check "GET /client-auth/shared-accounts" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/settings/whitelist" "$CLIENT_TOKEN")
check "GET /client-auth/settings/whitelist" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/tickets" "$CLIENT_TOKEN")
check "GET /client-auth/tickets" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/invoices" "$CLIENT_TOKEN")
check "GET /client-auth/invoices" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/logs/types" "$CLIENT_TOKEN")
check "GET /client-auth/logs/types" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/logs/packages" "$CLIENT_TOKEN")
check "GET /client-auth/logs/packages" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-auth/referral/code" "$CLIENT_TOKEN")
# This endpoint returns {"id":1,"code":"abc123",...} where code is the referral code, not an error code
REFERRAL_HAS_ID=$(printf '%s' "$RESP" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); print('yes' if 'id' in d or 'data' in d else 'no')" 2>/dev/null)
if [ "$REFERRAL_HAS_ID" = "yes" ]; then
  echo -e "  ${GREEN}✓ PASS${NC} GET /client-auth/referral/code"
  PASS=$((PASS+1))
else
  echo -e "  ${RED}✗ FAIL${NC} GET /client-auth/referral/code"
  FAIL=$((FAIL+1))
  ERRORS+=("GET /client-auth/referral/code")
fi

RESP=$(req GET "/api/v1/client-auth/referral/stats" "$CLIENT_TOKEN")
check "GET /client-auth/referral/stats" "$RESP" "ok"

# Unauthorized access
CODE=$(req_code GET "/api/v1/client-auth/me" "")
check_http "GET /client-auth/me without token (401)" "$CODE" "401"

# --- Admin: Clients ---
echo -e "\n${CYAN}[5] Admin: Clients${NC}"
RESP=$(req GET "/api/v1/clients" "$ADMIN_TOKEN")
check "GET /clients" "$RESP" "ok"

RESP=$(req GET "/api/v1/clients?limit=5&offset=0" "$ADMIN_TOKEN")
check "GET /clients (paginated)" "$RESP" "ok"

# Get first client ID
CLIENT_ID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); items=d.get('data',{}).get('items',d.get('data',[])); print(items[0]['id'] if items else 0)" 2>/dev/null || echo "0")
if [ "$CLIENT_ID" != "0" ]; then
  RESP=$(req GET "/api/v1/clients/$CLIENT_ID" "$ADMIN_TOKEN")
  check "GET /clients/:id" "$RESP" "ok"

  RESP=$(req GET "/api/v1/clients/$CLIENT_ID/orders" "$ADMIN_TOKEN")
  check "GET /clients/:id/orders" "$RESP" "ok"
fi

# --- Admin: Subscriptions ---
echo -e "\n${CYAN}[6] Admin: Subscriptions${NC}"
RESP=$(req GET "/api/v1/subscriptions" "$ADMIN_TOKEN")
check "GET /subscriptions" "$RESP" "ok"

SUB_ID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); items=d.get('data',{}).get('items',d.get('data',[])); print(items[0]['id'] if items else 0)" 2>/dev/null || echo "0")
if [ "$SUB_ID" != "0" ]; then
  RESP=$(req GET "/api/v1/subscriptions/$SUB_ID" "$ADMIN_TOKEN")
  check "GET /subscriptions/:id" "$RESP" "ok"
fi

# --- Admin: Plan Templates ---
echo -e "\n${CYAN}[7] Admin: Plan Templates${NC}"
RESP=$(req GET "/api/v1/plan-templates" "$ADMIN_TOKEN")
check "GET /plan-templates" "$RESP" "ok"

TMPL_ID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); items=d.get('data',{}).get('items',d.get('data',[])); print(items[0]['id'] if items else 0)" 2>/dev/null || echo "0")
if [ "$TMPL_ID" != "0" ]; then
  RESP=$(req GET "/api/v1/plan-templates/$TMPL_ID" "$ADMIN_TOKEN")
  check "GET /plan-templates/:id" "$RESP" "ok"

  RESP=$(req GET "/api/v1/plan-templates/$TMPL_ID/variants" "$ADMIN_TOKEN")
  check "GET /plan-templates/:id/variants" "$RESP" "ok"

  RESP=$(req GET "/api/v1/plan-templates/$TMPL_ID/accessible-nodes" "$ADMIN_TOKEN")
  check "GET /plan-templates/:id/accessible-nodes" "$RESP" "ok"
fi

# --- Admin: Plan Variants ---
echo -e "\n${CYAN}[8] Admin: Traffic Packs${NC}"
RESP=$(req GET "/api/v1/traffic-packs" "$ADMIN_TOKEN")
check "GET /traffic-packs" "$RESP" "ok"

# --- Admin: Promotions ---
echo -e "\n${CYAN}[9] Admin: Promotions${NC}"
RESP=$(req GET "/api/v1/promotions" "$ADMIN_TOKEN")
check "GET /promotions" "$RESP" "ok"

# --- Admin: Node Groups ---
echo -e "\n${CYAN}[10] Admin: Node Groups${NC}"
RESP=$(req GET "/api/v1/node-groups" "$ADMIN_TOKEN")
check "GET /node-groups" "$RESP" "ok"

# --- Admin: Node Servers ---
echo -e "\n${CYAN}[11] Admin: Node Servers${NC}"
RESP=$(req GET "/api/v1/node-servers" "$ADMIN_TOKEN")
check "GET /node-servers" "$RESP" "ok"

NS_ID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); items=d.get('data',{}).get('items',d.get('data',[])); print(items[0]['id'] if items else 0)" 2>/dev/null || echo "0")
if [ "$NS_ID" != "0" ]; then
  RESP=$(req GET "/api/v1/node-servers/$NS_ID" "$ADMIN_TOKEN")
  check "GET /node-servers/:id" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/status" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/status" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/binaries" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/binaries" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/services" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/services" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/services/process-status" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/services/process-status" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/status" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/status" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/inbound-configs" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/inbound-configs" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/outbound-configs" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/outbound-configs" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/protocols" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/protocols" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/certificates" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/certificates" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/rate-limit" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/rate-limit" "$RESP" "ok"

  # Geo rules are now globally managed (centered) and synced to nodes, so the
  # API lives under /api/v1/geo-rules/* (no node-servers/:id prefix).
  RESP=$(req GET "/api/v1/geo-rules/sources" "$ADMIN_TOKEN")
  check "GET /geo-rules/sources" "$RESP" "ok"

  RESP=$(req GET "/api/v1/geo-rules/files" "$ADMIN_TOKEN")
  check "GET /geo-rules/files" "$RESP" "ok"

  RESP=$(req GET "/api/v1/geo-rules/status" "$ADMIN_TOKEN")
  check "GET /geo-rules/status" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/share-domains" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/share-domains" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/warp" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/warp" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/xray/stats" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/xray/stats" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/xray/stats/users" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/xray/stats/users" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/online-users?limit=5&offset=0" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/online-users" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/metrics-history" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/metrics-history" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/traffic-usage" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/traffic-usage" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/traffic-history" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/traffic-history" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/tags" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/tags" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/degradation-status" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/degradation-status" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/sites" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/sites" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/config" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/config" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/sni-settings" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/sni-settings" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/l4-settings" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/l4-settings" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/log-settings" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/log-settings" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/proxy/nginx-settings" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/proxy/nginx-settings" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/caddy/sites" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/caddy/sites" "$RESP" "ok"

  RESP=$(req GET "/api/v1/node-servers/$NS_ID/nginx/sites" "$ADMIN_TOKEN")
  check "GET /node-servers/:id/nginx/sites" "$RESP" "ok"

  # Firewall injection tests
  RESP=$(req POST "/api/v1/node-servers/$NS_ID/xray/control/firewall/apply" "$ADMIN_TOKEN" '{"port_range":"80; rm -rf /","target_port":443,"interface":"eth0"}')
  check "POST firewall/apply (injection port_range rejected)" "$RESP" "error"

  RESP=$(req POST "/api/v1/node-servers/$NS_ID/xray/control/firewall/apply" "$ADMIN_TOKEN" '{"port_range":"80-443","target_port":443,"interface":"eth0; cat /etc/passwd"}')
  check "POST firewall/apply (injection interface rejected)" "$RESP" "error"

  RESP=$(req POST "/api/v1/node-servers/$NS_ID/xray/control/firewall/apply" "$ADMIN_TOKEN" '{"port_range":"$(whoami)","target_port":443,"interface":"eth0"}')
  check "POST firewall/apply (command substitution rejected)" "$RESP" "error"
fi

# --- Admin: Node Server Collections ---
echo -e "\n${CYAN}[12] Admin: Node Server Collections${NC}"
RESP=$(req GET "/api/v1/node-servers/online-summary" "$ADMIN_TOKEN")
check "GET /node-servers/online-summary" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/billable" "$ADMIN_TOKEN")
check "GET /node-servers/billable" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/dashboard" "$ADMIN_TOKEN")
check "GET /node-servers/dashboard" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/pending-agent-updates" "$ADMIN_TOKEN")
check "GET /node-servers/pending-agent-updates" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/ca-cert" "$ADMIN_TOKEN")
check "GET /node-servers/ca-cert" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/traffic/stats" "$ADMIN_TOKEN")
check "GET /node-servers/traffic/stats" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/metrics/aggregated" "$ADMIN_TOKEN")
check "GET /node-servers/metrics/aggregated" "$RESP" "ok"

RESP=$(req GET "/api/v1/node-servers/speed-test-results" "$ADMIN_TOKEN")
check "GET /node-servers/speed-test-results" "$RESP" "ok"

# --- Admin: Certificates ---
echo -e "\n${CYAN}[13] Admin: Certificates${NC}"
RESP=$(req GET "/api/v1/certificates" "$ADMIN_TOKEN")
check "GET /certificates" "$RESP" "ok"

RESP=$(req GET "/api/v1/certificates/detect-ip" "$ADMIN_TOKEN")
check "GET /certificates/detect-ip" "$RESP" "ok"

RESP=$(req GET "/api/v1/certificates/check-port?port=80" "$ADMIN_TOKEN")
check "GET /certificates/check-port" "$RESP" "ok"

# --- Admin: Cert Channels ---
echo -e "\n${CYAN}[14] Admin: Cert Channels${NC}"
RESP=$(req GET "/api/v1/cert-channels" "$ADMIN_TOKEN")
check "GET /cert-channels" "$RESP" "ok"

RESP=$(req GET "/api/v1/cert-channels/providers" "$ADMIN_TOKEN")
check "GET /cert-channels/providers" "$RESP" "ok"

# --- Admin: ACME Accounts ---
echo -e "\n${CYAN}[15] Admin: ACME Accounts${NC}"
RESP=$(req GET "/api/v1/acme-accounts" "$ADMIN_TOKEN")
check "GET /acme-accounts" "$RESP" "ok"

RESP=$(req GET "/api/v1/acme-accounts/default" "$ADMIN_TOKEN")
check "GET /acme-accounts/default" "$RESP" "ok"

# --- Admin: CA Center ---
echo -e "\n${CYAN}[16] Admin: CA Center${NC}"
RESP=$(req GET "/api/v1/ca/status" "$ADMIN_TOKEN")
check "GET /ca/status" "$RESP" "ok"

RESP=$(req GET "/api/v1/ca/agent-certs" "$ADMIN_TOKEN")
check "GET /ca/agent-certs" "$RESP" "ok"

RESP=$(req GET "/api/v1/ca/revocations" "$ADMIN_TOKEN")
check "GET /ca/revocations" "$RESP" "ok"

RESP=$(req GET "/api/v1/ca/distribution-queue" "$ADMIN_TOKEN")
check "GET /ca/distribution-queue" "$RESP" "ok"

# --- Admin: Charge Sites ---
echo -e "\n${CYAN}[17] Admin: Charge Sites${NC}"
RESP=$(req GET "/api/v1/charge-sites" "$ADMIN_TOKEN")
check "GET /charge-sites" "$RESP" "ok"

# --- Admin: Charge Orders ---
echo -e "\n${CYAN}[18] Admin: Charge Orders${NC}"
RESP=$(req GET "/api/v1/charge-orders" "$ADMIN_TOKEN")
check "GET /charge-orders" "$RESP" "ok"

# --- Admin: Charge Cards ---
echo -e "\n${CYAN}[19] Admin: Charge Cards${NC}"
RESP=$(req GET "/api/v1/charge-cards" "$ADMIN_TOKEN")
check "GET /charge-cards" "$RESP" "ok"

# --- Admin: Client Balances ---
echo -e "\n${CYAN}[20] Admin: Client Balances${NC}"
RESP=$(req GET "/api/v1/client-balances" "$ADMIN_TOKEN")
check "GET /client-balances" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-balances/transactions" "$ADMIN_TOKEN")
check "GET /client-balances/transactions" "$RESP" "ok"

# --- Admin: Callback Events ---
echo -e "\n${CYAN}[21] Admin: Callback Events${NC}"
RESP=$(req GET "/api/v1/charge-callback-events" "$ADMIN_TOKEN")
check "GET /charge-callback-events" "$RESP" "ok"

RESP=$(req GET "/api/v1/charge-callback-retries" "$ADMIN_TOKEN")
check "GET /charge-callback-retries" "$RESP" "ok"

# --- Admin: Referral ---
echo -e "\n${CYAN}[22] Admin: Referral${NC}"
RESP=$(req GET "/api/v1/referral-withdrawals" "$ADMIN_TOKEN")
check "GET /referral-withdrawals" "$RESP" "ok"

RESP=$(req GET "/api/v1/referral/stats" "$ADMIN_TOKEN")
check "GET /referral/stats" "$RESP" "ok"

RESP=$(req GET "/api/v1/referral/records" "$ADMIN_TOKEN")
check "GET /referral/records" "$RESP" "ok"

RESP=$(req GET "/api/v1/referral/config" "$ADMIN_TOKEN")
check "GET /referral/config" "$RESP" "ok"

# --- Admin: Tickets ---
echo -e "\n${CYAN}[23] Admin: Tickets${NC}"
RESP=$(req GET "/api/v1/tickets" "$ADMIN_TOKEN")
check "GET /tickets" "$RESP" "ok"

# --- Admin: Gifts ---
echo -e "\n${CYAN}[24] Admin: Gifts${NC}"
RESP=$(req GET "/api/v1/client-gifts" "$ADMIN_TOKEN")
check "GET /client-gifts" "$RESP" "ok"

RESP=$(req GET "/api/v1/client-gifts/search-clients" "$ADMIN_TOKEN")
check "GET /client-gifts/search-clients" "$RESP" "ok"

# --- Admin: Articles ---
echo -e "\n${CYAN}[25] Admin: Articles${NC}"
RESP=$(req GET "/api/v1/articles" "$ADMIN_TOKEN")
check "GET /articles" "$RESP" "ok"

# --- Admin: Shared Accounts ---
echo -e "\n${CYAN}[26] Admin: Shared Accounts${NC}"
RESP=$(req GET "/api/v1/shared-accounts" "$ADMIN_TOKEN")
check "GET /shared-accounts" "$RESP" "ok"

# --- Admin: IP Whitelists ---
echo -e "\n${CYAN}[27] Admin: IP Whitelists${NC}"
RESP=$(req GET "/api/v1/ip-whitelists" "$ADMIN_TOKEN")
check "GET /ip-whitelists" "$RESP" "ok"

# --- Admin: OAuth Bindings ---
echo -e "\n${CYAN}[28] Admin: OAuth Bindings${NC}"
RESP=$(req GET "/api/v1/client-oauth-bindings" "$ADMIN_TOKEN")
check "GET /client-oauth-bindings" "$RESP" "ok"

# --- Admin: Subscription Formats ---
echo -e "\n${CYAN}[29] Admin: Subscription Formats${NC}"
RESP=$(req GET "/api/v1/subscription-formats" "$ADMIN_TOKEN")
check "GET /subscription-formats (retired)" "$RESP" "error"

# --- Admin: Subscription Endpoints ---
echo -e "\n${CYAN}[30] Admin: Subscription Endpoints${NC}"
RESP=$(req GET "/api/v1/subscription-endpoints" "$ADMIN_TOKEN")
check "GET /subscription-endpoints" "$RESP" "ok"

# --- Admin: Mihomo Configs ---
echo -e "\n${CYAN}[31] Admin: Mihomo Configs${NC}"
RESP=$(req GET "/api/v1/mihomo/configs" "$ADMIN_TOKEN")
check "GET /mihomo/configs" "$RESP" "ok"

# --- Admin: Inbounds ---
echo -e "\n${CYAN}[32] Admin: Inbounds${NC}"
RESP=$(req GET "/api/v1/inbounds-with-server" "$ADMIN_TOKEN")
check "GET /inbounds-with-server" "$RESP" "ok"

# --- Admin: CFIP ---
echo -e "\n${CYAN}[33] Admin: CFIP${NC}"
RESP=$(req GET "/api/v1/cfip/entries" "$ADMIN_TOKEN")
check "GET /cfip/entries" "$RESP" "ok"

# --- Admin: Regions ---
echo -e "\n${CYAN}[34] Admin: Regions${NC}"
RESP=$(req GET "/api/v1/regions/overview" "$ADMIN_TOKEN")
check "GET /regions/overview" "$RESP" "ok"

# --- Admin: Node Audit Logs ---
echo -e "\n${CYAN}[35] Admin: Node Audit Logs${NC}"
RESP=$(req GET "/api/v1/node-audit-logs" "$ADMIN_TOKEN")
check "GET /node-audit-logs" "$RESP" "ok"

# --- Admin: Metrics ---
echo -e "\n${CYAN}[36] Admin: Metrics${NC}"
RESP=$(req GET "/api/v1/metrics/overview" "$ADMIN_TOKEN")
check "GET /metrics/overview" "$RESP" "ok"

RESP=$(req GET "/api/v1/metrics/services" "$ADMIN_TOKEN")
check "GET /metrics/services (retired)" "$RESP" "error"

RESP=$(req GET "/api/v1/metrics/nodes" "$ADMIN_TOKEN")
check "GET /metrics/nodes" "$RESP" "ok"

RESP=$(req GET "/api/v1/metrics/certificates" "$ADMIN_TOKEN")
check "GET /metrics/certificates" "$RESP" "ok"

# --- Admin: Security ---
echo -e "\n${CYAN}[37] Admin: Security${NC}"
RESP=$(req GET "/api/v1/security/audit-events" "$ADMIN_TOKEN")
check "GET /security/audit-events" "$RESP" "ok"

RESP=$(req GET "/api/v1/security/alerts" "$ADMIN_TOKEN")
check "GET /security/alerts (route not registered)" "$RESP" "error"

RESP=$(req GET "/api/v1/security/alert-channels" "$ADMIN_TOKEN")
check "GET /security/alert-channels (route not registered)" "$RESP" "error"

# --- Admin: Backups ---
echo -e "\n${CYAN}[38] Admin: Backups${NC}"
RESP=$(req GET "/api/v1/backups/configs" "$ADMIN_TOKEN")
check "GET /backups/configs (route not registered)" "$RESP" "error"

RESP=$(req GET "/api/v1/backups/database" "$ADMIN_TOKEN")
check "GET /backups/database (retired)" "$RESP" "error"

RESP=$(req GET "/api/v1/backups/retention-policies" "$ADMIN_TOKEN")
check "GET /backups/retention-policies (route not registered)" "$RESP" "error"

RESP=$(req GET "/api/v1/backups/cleanup-jobs" "$ADMIN_TOKEN")
check "GET /backups/cleanup-jobs (retired)" "$RESP" "error"

RESP=$(req GET "/api/v1/backups/list" "$ADMIN_TOKEN")
check "GET /backups/list" "$RESP" "ok"

RESP=$(req GET "/api/v1/backups/schedules" "$ADMIN_TOKEN")
check "GET /backups/schedules" "$RESP" "ok"

RESP=$(req GET "/api/v1/backups/storages" "$ADMIN_TOKEN")
check "GET /backups/storages" "$RESP" "ok"

# --- Admin: System ---
echo -e "\n${CYAN}[39] Admin: System${NC}"
RESP=$(req GET "/api/v1/system/stats" "$ADMIN_TOKEN")
check "GET /system/stats" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/bootstrap-state" "$ADMIN_TOKEN")
check "GET /system/bootstrap-state" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/releases" "$ADMIN_TOKEN")
check "GET /system/releases (not implemented)" "$RESP" "error"

RESP=$(req GET "/api/v1/system/update-jobs" "$ADMIN_TOKEN")
check "GET /system/update-jobs (not implemented)" "$RESP" "error"

RESP=$(req GET "/api/v1/system/panel-config" "$ADMIN_TOKEN")
check "GET /system/panel-config" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/notification-config" "$ADMIN_TOKEN")
check "GET /system/notification-config" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/github-config" "$ADMIN_TOKEN")
check "GET /system/github-config" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/db-config" "$ADMIN_TOKEN")
check "GET /system/db-config" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/subscription-billing-templates" "$ADMIN_TOKEN")
check "GET /system/subscription-billing-templates" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/notification-templates" "$ADMIN_TOKEN")
check "GET /system/notification-templates" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/reality-targets" "$ADMIN_TOKEN")
check "GET /system/reality-targets" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/settings?prefix=panel" "$ADMIN_TOKEN")
check "GET /system/settings?prefix=panel" "$RESP" "ok"

RESP=$(req GET "/api/v1/system/encryption-key-status" "$ADMIN_TOKEN")
check "GET /system/encryption-key-status" "$RESP" "ok"

# --- Admin: Log Management ---
echo -e "\n${CYAN}[40] Admin: Log Management${NC}"
RESP=$(req GET "/api/v1/log-manage/status" "$ADMIN_TOKEN")
check "GET /log-manage/status" "$RESP" "ok"

RESP=$(req GET "/api/v1/log-manage/policies" "$ADMIN_TOKEN")
check "GET /log-manage/policies" "$RESP" "ok"

RESP=$(req GET "/api/v1/log-manage/log-output" "$ADMIN_TOKEN")
check "GET /log-manage/log-output" "$RESP" "ok"

RESP=$(req GET "/api/v1/log-manage/core-log-output" "$ADMIN_TOKEN")
check "GET /log-manage/core-log-output" "$RESP" "ok"

RESP=$(req GET "/api/v1/log-manage/writers" "$ADMIN_TOKEN")
check "GET /log-manage/writers" "$RESP" "ok"

# --- Admin: Portal ---
echo -e "\n${CYAN}[41] Admin: Portal${NC}"
RESP=$(req GET "/api/v1/portal/templates" "$ADMIN_TOKEN")
check "GET /portal/templates" "$RESP" "ok"

# --- External Nodes ---
echo -e "\n${CYAN}[42] External Nodes${NC}"
RESP=$(req GET "/api/v1/external-nodes/sources" "$ADMIN_TOKEN")
check "GET /external-nodes/sources" "$RESP" "ok"

# --- Auth: Users ---
echo -e "\n${CYAN}[43] Admin: Users${NC}"
RESP=$(req GET "/api/v1/users" "$ADMIN_TOKEN")
check "GET /users" "$RESP" "ok"

# --- Permission Tests ---
echo -e "\n${CYAN}[44] Permission Tests${NC}"
CODE=$(req_code GET "/api/v1/clients" "")
check_http "GET /clients without token (401)" "$CODE" "401"

CODE=$(req_code GET "/api/v1/system/stats" "")
check_http "GET /system/stats without token (401)" "$CODE" "401"

CODE=$(req_code GET "/api/v1/node-servers" "")
check_http "GET /node-servers without token (401)" "$CODE" "401"

CODE=$(req_code GET "/api/v1/certificates" "")
check_http "GET /certificates without token (401)" "$CODE" "401"

# --- Method Not Allowed ---
echo -e "\n${CYAN}[45] Method / Route Tests${NC}"
CODE=$(req_code GET "/api/v1/nonexistent" "$ADMIN_TOKEN")
check_http "GET /nonexistent (404)" "$CODE" "404"

# --- Summary ---
echo ""
echo "================================================"
echo "  TEST SUMMARY"
echo "================================================"
echo -e "  ${GREEN}PASS: $PASS${NC}"
echo -e "  ${RED}FAIL: $FAIL${NC}"
echo "  TOTAL: $((PASS+FAIL))"
echo "================================================"

if [ ${#ERRORS[@]} -gt 0 ]; then
  echo -e "\n${RED}Failed tests:${NC}"
  for err in "${ERRORS[@]}"; do
    echo "  - $err"
  done
fi

exit $FAIL
