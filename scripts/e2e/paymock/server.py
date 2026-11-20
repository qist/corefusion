#!/usr/bin/env python3
"""CoreFusion 模拟支付站点（e2e 专用，零依赖）。

模拟真实支付网关的完整链路，用于验证面板的「充值站点」支付：

    面板/门户 --POST /api/create-order--> 本站（校验 API Key 签名，返回 payment_url）
    浏览器   --GET  /pay?trade_no=...--> 收银台（可选渠道：支付宝/微信/QQ/USDT/银联）
    收银台   --POST /api/pay----------> 本站按对接契约回调面板 notify_url
    面板     <----回调验签/金额校验/入账---- 本站记录面板返回的状态与响应体

启动：
    PAYMOCK_API_KEY=xxx PAYMOCK_CALLBACK_SECRET=yyy python3 server.py
可选环境变量：
    PAYMOCK_ADDR            监听地址，默认 0.0.0.0:18081
    PAYMOCK_PUBLIC_URL      生成 payment_url 用的对外地址，默认 http://127.0.0.1:18081
    PAYMOCK_STATE_FILE      订单/回调记录落盘路径（默认同目录 state.json）

接口：
    POST /api/create-order  创建订单（验签）
    GET  /pay?trade_no=     收银台
    POST /api/pay           模拟支付（body: trade_no, method, mode=success|fail）
    POST /api/retry         重放上一次回调（验幂等）
    GET  /api/orders        订单与回调记录（JSON）
    GET  /admin             可视化观测页
    GET  /health            健康检查
"""

import hashlib
import hmac
import json
import os
import re
import secrets
import tempfile
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ADDR = os.environ.get("PAYMOCK_ADDR", "0.0.0.0:18081")
PUBLIC_URL = os.environ.get("PAYMOCK_PUBLIC_URL", "http://127.0.0.1:18081").rstrip("/")
API_KEY = os.environ.get("PAYMOCK_API_KEY", "")
CALLBACK_SECRET = os.environ.get("PAYMOCK_CALLBACK_SECRET", "")
# 默认落到临时目录：state.json 是运行态数据，不应出现在仓库里。
STATE_FILE = os.environ.get(
    "PAYMOCK_STATE_FILE", os.path.join(tempfile.gettempdir(), "paymock-state.json")
)

METHODS = [
    ("alipay", "支付宝", "#1677ff"),
    ("wechat", "微信支付", "#07c160"),
    ("qqpay", "QQ 钱包", "#12b7f5"),
    ("usdt", "USDT (TRC20)", "#26a17b"),
    ("bank", "银联/网银", "#d4380d"),
]

_lock = threading.Lock()
_orders = {}
_seq = 0


def dumps(obj):
    """与 Go json.Marshal(map) 对齐：键排序 + 紧凑分隔符 + 不转义中文。"""
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def load_state():
    global _orders, _seq
    try:
        with open(STATE_FILE, "r", encoding="utf-8") as f:
            data = json.load(f)
        _orders = data.get("orders", {})
        _seq = int(data.get("seq", 0))
    except Exception:
        _orders, _seq = {}, 0


def save_state():
    tmp = STATE_FILE + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump({"orders": _orders, "seq": _seq}, f, ensure_ascii=False, indent=1)
    os.replace(tmp, STATE_FILE)


def sign_map(payload, secret):
    body = {k: v for k, v in payload.items() if k != "sign"}
    return hmac.new(secret.encode(), dumps(body).encode(), hashlib.sha256).hexdigest()


def verify_request(raw, secret):
    """用「原始字节剔除 sign 成员」计算 HMAC，与面板签名时 marshal 的字节一致。"""
    m = re.search(rb',\s*"sign"\s*:\s*"[0-9a-fA-F]*"', raw)
    if m:
        stripped = raw[: m.start()] + raw[m.end():]
    else:
        m = re.search(rb'"sign"\s*:\s*"[0-9a-fA-F]*"\s*,', raw)
        if not m:
            return False
        stripped = raw[: m.start()] + raw[m.end():]
    try:
        sent = json.loads(raw).get("sign", "")
    except Exception:
        return False
    want = hmac.new(secret.encode(), stripped, hashlib.sha256).hexdigest()
    return hmac.compare_digest(want, sent)


def next_trade_suffix():
    global _seq
    with _lock:
        _seq += 1
        return _seq


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "PayMock/1.0"

    def log_message(self, fmt, *args):
        print("  %s %s" % (self.address_string(), fmt % args), flush=True)

    # ---------------- helpers ----------------

    def send_body(self, code, body, ctype="application/json; charset=utf-8"):
        if isinstance(body, str):
            body = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def send_json(self, code, obj):
        self.send_body(code, json.dumps(obj, ensure_ascii=False))

    def read_json(self):
        n = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(n) if n else b""
        if not raw:
            return {}, b""
        try:
            return json.loads(raw), raw
        except Exception:
            return None, raw

    # ---------------- routes ----------------

    def do_GET(self):
        path = self.path.split("?")[0]
        query = {}
        if "?" in self.path:
            for pair in self.path.split("?", 1)[1].split("&"):
                if "=" in pair:
                    k, v = pair.split("=", 1)
                    query[k] = urllib.request.unquote(v)

        if path == "/health":
            return self.send_json(200, {"ok": True, "orders": len(_orders), "time": int(time.time())})
        if path == "/pay":
            return self.page_cashier(query.get("trade_no", ""))
        if path == "/admin":
            return self.page_admin()
        if path == "/api/orders":
            with _lock:
                items = sorted(_orders.values(), key=lambda o: o.get("created", 0), reverse=True)
            return self.send_json(200, {"ok": True, "total": len(items), "orders": items})
        if path.startswith("/api/orders/"):
            trade_no = path.split("/api/orders/", 1)[1]
            with _lock:
                order = _orders.get(trade_no)
            if not order:
                return self.send_json(404, {"ok": False, "error": "order not found"})
            return self.send_json(200, {"ok": True, "order": order})
        return self.send_json(404, {"ok": False, "error": "not found"})

    def do_POST(self):
        path = self.path.split("?")[0]

        if path == "/api/create-order":
            return self.api_create_order()
        if path == "/api/pay":
            return self.api_pay()
        if path == "/api/retry":
            return self.api_retry()
        return self.send_json(404, {"ok": False, "error": "not found"})

    def api_create_order(self):
        data, raw = self.read_json()
        if data is None:
            return self.send_json(400, {"ok": False, "error": "invalid json"})
        if API_KEY and not verify_request(raw, API_KEY):
            return self.send_json(401, {"ok": False, "error": "invalid signature"})

        trade_no = str(data.get("trade_no") or "").strip()
        if not trade_no:
            return self.send_json(400, {"ok": False, "error": "trade_no is required"})
        amount = data.get("amount")
        if not isinstance(amount, int) or amount <= 0:
            return self.send_json(400, {"ok": False, "error": "amount must be a positive integer (minor unit)"})

        order = {
            "trade_no": trade_no,
            "amount": amount,
            "amount_yuan": round(amount / 100.0, 2),
            "currency": data.get("currency") or "CNY",
            "subject": data.get("subject") or "",
            "client_email": data.get("client_email") or "",
            "notify_url": data.get("notify_url") or "",
            "return_url": data.get("return_url") or "",
            "created": time.time(),
            "expires_in": 900,
            "status": "pending",
            "method": "",
            "request": data,
            "callbacks": [],
        }
        with _lock:
            _orders[trade_no] = order
        save_state()
        print("[create-order] %s amount=%s %s" % (trade_no, amount, order["currency"]), flush=True)
        return self.send_json(
            200,
            {
                "ok": True,
                "payment_url": "%s/pay?trade_no=%s" % (PUBLIC_URL, urllib.request.quote(trade_no)),
                "expires_in": order["expires_in"],
            },
        )

    def api_pay(self):
        data, _ = self.read_json()
        if not data:
            return self.send_json(400, {"ok": False, "error": "invalid json"})
        trade_no = str(data.get("trade_no") or "")
        method = str(data.get("method") or "alipay")
        mode = str(data.get("mode") or "success")
        channel = next((m for m in METHODS if m[0] == method), None)
        if not channel:
            return self.send_json(400, {"ok": False, "error": "unsupported method: %s" % method})

        with _lock:
            order = _orders.get(trade_no)
            if not order:
                return self.send_json(404, {"ok": False, "error": "order not found"})
            if order["status"] == "paid":
                return self.send_json(409, {"ok": False, "error": "order already paid"})
            order["method"] = method
            order["paid_at"] = time.time()

        status = "failed" if mode == "fail" else "paid"
        # 负向测试：mode=bad-sign 伪造签名，amount 覆盖回调金额（验证面板金额锁定）
        override = data.get("amount")
        if not isinstance(override, int):
            override = None
        cb = self.deliver_callback(order, status, amount_override=override, bad_sign=(mode == "bad-sign"))
        with _lock:
            if order["status"] != "paid":
                order["status"] = "failed" if status == "failed" else ("paid" if cb["ok"] else "callback_failed")
        save_state()
        return self.send_json(200, {"ok": cb["ok"], "status": order["status"], "callback": cb})

    def api_retry(self):
        data, _ = self.read_json()
        trade_no = str((data or {}).get("trade_no") or "")
        with _lock:
            order = _orders.get(trade_no)
            if not order:
                return self.send_json(404, {"ok": False, "error": "order not found"})
        cb = self.deliver_callback(order, "paid")
        save_state()
        return self.send_json(200, {"ok": cb["ok"], "callback": cb})

    # ---------------- callback ----------------

    def deliver_callback(self, order, status, amount_override=None, bad_sign=False):
        """按对接契约回调面板：body_hash(去 sign) + 8 字段 HMAC，sign 追加在末尾。

        amount_override / bad_sign 仅供负向测试：验证面板的金额锁定与验签。
        """
        nonce = secrets.token_hex(8)
        amount = amount_override if amount_override is not None else order["amount"]
        body = {
            "trade_no": order["trade_no"],
            "site_trade_no": "MOCK" + order["trade_no"][-10:] + secrets.token_hex(2),
            "status": status,
            "amount": amount,
            "currency": order["currency"],
            "timestamp": int(time.time()),
            "nonce": nonce,
            "paid_currency": order["currency"],
            "paid_amount": amount,
            "method": order["method"] or "alipay",
            "paid_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        raw_no_sign = dumps(body)
        body_hash = hashlib.sha256(raw_no_sign.encode()).hexdigest()
        sign_payload = {
            "amount": body["amount"],
            "body_hash": body_hash,
            "currency": body["currency"],
            "nonce": nonce,
            "site_trade_no": body["site_trade_no"],
            "timestamp": body["timestamp"],
            "trade_no": body["trade_no"],
            "version": "v1",
        }
        sign = sign_map(sign_payload, CALLBACK_SECRET)
        if bad_sign:
            sign = "0" * 64
        raw_body = raw_no_sign[:-1] + ',"sign":"%s"}' % sign

        started = time.time()
        code, resp = 0, ""
        try:
            req = urllib.request.Request(
                order["notify_url"],
                data=raw_body.encode(),
                headers={"Content-Type": "application/json", "X-Request-ID": "mock-" + nonce},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=15) as r:
                code = r.status
                resp = r.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as e:
            code = e.code
            resp = e.read().decode("utf-8", "replace")
        except Exception as e:  # noqa: BLE001
            code, resp = 0, "callback failed: %s" % e

        record = {
            "at": time.time(),
            "nonce": nonce,
            "status": status,
            "amount": body["amount"],
            "currency": body["currency"],
            "bad_sign": bad_sign,
            "http_code": code,
            "ok": code == 200,
            "response": resp[:1000],
            "elapsed_ms": int((time.time() - started) * 1000),
            "request_body": raw_body,
            "sign": sign,
        }
        with _lock:
            order.setdefault("callbacks", []).append(record)
        print("[callback] %s status=%s http=%s %dms" % (order["trade_no"], status, code, record["elapsed_ms"]), flush=True)
        return record

    # ---------------- pages ----------------

    def page_cashier(self, trade_no):
        with _lock:
            order = _orders.get(trade_no)
        if not order:
            return self.send_body(404, "<h1>订单不存在</h1>", "text/html; charset=utf-8")
        if order["status"] == "paid":
            return self.send_body(200, render_cashier_done(order), "text/html; charset=utf-8")

        methods_html = "".join(
            '<button class="m" data-m="%s" style="--c:%s"><span>%s</span></button>' % (mid, color, name)
            for mid, name, color in METHODS
        )
        html = (CASHIER_HTML
                .replace("{{TRADE_NO}}", esc(order["trade_no"]))
                .replace("{{AMOUNT}}", esc(order["amount_yuan"]))
                .replace("{{CURRENCY}}", esc(order["currency"]))
                .replace("{{SUBJECT}}", esc(order["subject"]))
                .replace("{{EMAIL}}", esc(order["client_email"]))
                .replace("{{METHODS}}", methods_html)
                .replace("{{TRADE_JSON}}", json.dumps(trade_no)))
        return self.send_body(200, html, "text/html; charset=utf-8")

    def page_admin(self):
        return self.send_body(200, ADMIN_HTML, "text/html; charset=utf-8")


def esc(v):
    """最小 HTML 转义（这些值来自面板参数，必须转义）。"""
    return (str(v).replace("&", "&amp;").replace("<", "&lt;")
            .replace(">", "&gt;").replace('"', "&quot;"))


def render_cashier_done(order):
    return (CASHIER_DONE_HTML
            .replace("{{TRADE_NO}}", esc(order["trade_no"]))
            .replace("{{AMOUNT}}", esc(order["amount_yuan"]))
            .replace("{{CURRENCY}}", esc(order["currency"]))
            .replace("{{METHOD}}", esc(order.get("method") or "-")))


CASHIER_HTML = """<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>模拟收银台</title>
<style>
*{box-sizing:border-box}body{margin:0;font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#f4f6fa;display:flex;justify-content:center;padding:48px 16px;color:#111}
.card{background:#fff;border-radius:16px;box-shadow:0 10px 40px rgba(16,24,40,.10);width:440px;overflow:hidden}
.head{padding:22px 28px;border-bottom:1px solid #eef0f4;display:flex;justify-content:space-between;align-items:center}
.badge{font-size:12px;background:#eef2ff;color:#4f46e5;padding:4px 10px;border-radius:999px}
.body{padding:22px 28px}
.row{display:flex;justify-content:space-between;font-size:13px;color:#667085;margin:6px 0}
.row b{color:#111;font-weight:600;word-break:break-all}
.amount{font-size:34px;font-weight:700;margin:14px 0 4px}
.amount small{font-size:14px;font-weight:500;color:#667085;margin-left:4px}
label.lbl{display:block;font-size:13px;color:#344054;margin:18px 0 8px;font-weight:600}
.methods{display:grid;grid-template-columns:1fr 1fr;gap:10px}
.m{display:flex;align-items:center;justify-content:center;gap:8px;height:48px;border:1.5px solid #e4e7ec;background:#fff;border-radius:10px;font-size:14px;cursor:pointer;transition:.15s;position:relative}
.m:hover{border-color:var(--c)}
.m.active{border-color:var(--c);box-shadow:0 0 0 3px color-mix(in srgb,var(--c) 16%,transparent);font-weight:600}
.m span:before{content:"";display:inline-block;width:8px;height:8px;border-radius:50%;background:var(--c);margin-right:8px;vertical-align:1px}
.pay{width:100%;margin-top:20px;height:48px;border:0;border-radius:10px;background:#4f46e5;color:#fff;font-size:16px;font-weight:600;cursor:pointer}
.pay:disabled{opacity:.55;cursor:not-allowed}
.hint{font-size:12px;color:#98a2b3;margin-top:12px;text-align:center}
.result{margin-top:16px;padding:12px;border-radius:10px;font-size:12px;white-space:pre-wrap;word-break:break-all;display:none}
.result.ok{display:block;background:#ecfdf3;color:#027a48;border:1px solid #abefc6}
.result.err{display:block;background:#fef3f2;color:#b42318;border:1px solid #fecdca}
</style></head><body>
<div class="card">
  <div class="head"><strong>模拟收银台</strong><span class="badge">TEST MODE</span></div>
  <div class="body">
    <div class="row"><span>订单号</span><b>{{TRADE_NO}}</b></div>
    <div class="row"><span>商品</span><b>{{SUBJECT}}</b></div>
    <div class="row"><span>付款人</span><b>{{EMAIL}}</b></div>
    <div class="amount">¥ {{AMOUNT}} <small>{{CURRENCY}}</small></div>
    <label class="lbl">选择支付方式</label>
    <div class="methods">{{METHODS}}</div>
    <button class="pay" id="payBtn" disabled>请先选择支付方式</button>
    <div class="hint">仅用于联调，不产生真实扣款</div>
    <div class="result" id="result"></div>
  </div>
</div>
<script>
var tradeNo = {{TRADE_JSON}};
var method = '';
document.querySelectorAll('.m').forEach(function(b){
  b.addEventListener('click', function(){
    document.querySelectorAll('.m').forEach(function(x){ x.classList.remove('active') });
    b.classList.add('active');
    method = b.dataset.m;
    var btn = document.getElementById('payBtn');
    btn.disabled = false;
    btn.textContent = '使用 ' + b.querySelector('span').textContent + ' 支付';
  });
});
document.getElementById('payBtn').addEventListener('click', function(){
  var btn = this; btn.disabled = true; btn.textContent = '正在跳转支付…';
  fetch('/api/pay', {method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({trade_no: tradeNo, method: method, mode: 'success'})})
  .then(function(r){ return r.json() })
  .then(function(d){
    var box = document.getElementById('result');
    var cb = d.callback || {};
    var ok = d.ok && cb.ok;
    box.className = 'result ' + (ok ? 'ok' : 'err');
    box.textContent = (ok ? '支付成功，已回调面板并确认到账\\n' : '支付完成但面板未确认\\n')
      + '渠道: ' + method
      + '\\n面板 HTTP: ' + (cb.http_code || '-')
      + '\\n耗时: ' + (cb.elapsed_ms || 0) + 'ms'
      + '\\n响应: ' + ((cb.response || '').slice(0, 300));
    btn.textContent = ok ? '已完成' : '重试';
    btn.disabled = false;
  }).catch(function(e){
    var box = document.getElementById('result');
    box.className = 'result err'; box.textContent = '请求失败: ' + e;
    btn.disabled = false; btn.textContent = '重试';
  });
});
</script></body></html>"""

CASHIER_DONE_HTML = """<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>支付完成</title>
<style>body{font-family:system-ui;background:#f4f6fa;display:flex;justify-content:center;padding:60px}
.card{background:#fff;border-radius:16px;padding:32px 36px;box-shadow:0 10px 40px rgba(16,24,40,.1);text-align:center}
.ok{width:56px;height:56px;border-radius:50%;background:#ecfdf3;color:#12b76a;font-size:30px;line-height:56px;margin:0 auto 16px}
</style></head><body><div class="card"><div class="ok">✓</div>
<h2 style="margin:0 0 8px">支付成功</h2>
<div style="color:#667085;font-size:14px">订单 {{TRADE_NO}}</div>
<div style="font-size:28px;font-weight:700;margin:12px 0">¥ {{AMOUNT}} <small style="font-size:13px">{{CURRENCY}}</small></div>
<div style="color:#98a2b3;font-size:12px">渠道: {{METHOD}} · 可关闭本页返回门户</div>
</div></body></html>"""

ADMIN_HTML = """<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>PayMock 观测台</title>
<style>body{font-family:system-ui;margin:0;background:#f4f6fa;padding:28px;color:#111}
h1{font-size:18px;margin:0 0 16px}table{width:100%%;border-collapse:collapse;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(16,24,40,.06);font-size:12px}
th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #eef0f4;vertical-align:top}th{background:#fafbfc;color:#667085;font-weight:600}
code{background:#f2f4f7;padding:2px 6px;border-radius:4px;font-size:11px}.ok{color:#027a48}.bad{color:#b42318}
pre{margin:4px 0;white-space:pre-wrap;word-break:break-all;font-size:11px;color:#475467}</style></head><body>
<h1>PayMock 订单与回调记录</h1><table><thead><tr><th>订单</th><th>金额</th><th>状态</th><th>渠道</th><th>回调</th></tr></thead><tbody id="rows"></tbody></table>
<script>
function load(){
 fetch('/api/orders').then(r=>r.json()).then(d=>{
  document.getElementById('rows').innerHTML = d.orders.map(function(o){
   var cbs = (o.callbacks||[]).map(function(c){
     return '<div class="'+(c.ok?'ok':'bad')+'">#'+(c.http_code)+' '+c.elapsed_ms+'ms '+c.status+'</div><pre>'+ (c.response||'').replace(/</g,'&lt;') +'</pre>';
   }).join('') || '-';
   return '<tr><td><code>'+o.trade_no+'</code><br><span style="color:#98a2b3">'+o.client_email+'</span></td>'
     +'<td>'+o.amount_yuan+' '+o.currency+'</td><td>'+o.status+'</td><td>'+(o.method||'-')+'</td><td>'+cbs+'</td></tr>';
  }).join('');
 });
}
load(); setInterval(load, 3000);
</script></body></html>"""


def main():
    load_state()
    host, _, port = ADDR.rpartition(":")
    if not host:
        host = "0.0.0.0"
    srv = ThreadingHTTPServer((host, int(port)), Handler)
    srv.daemon_threads = True
    print("PayMock listening on http://%s:%s (public=%s)" % (host, port, PUBLIC_URL), flush=True)
    print("  cashier: %s/pay?trade_no=<trade_no>" % PUBLIC_URL, flush=True)
    print("  console: %s/admin" % PUBLIC_URL, flush=True)
    srv.serve_forever()


if __name__ == "__main__":
    main()
