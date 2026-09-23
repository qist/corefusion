// Command mockchargesite 是**测试用的模拟充值站点**（假收银台）✓，
// 用来端到端验证 CoreFusion 的充值链路 ✓。
//
// ⚠ 它是**独立测试模块**（同目录有独立 go.mod ✓）✗：
// 不参与产品构建 ✓、产品代码里没有任何"为测试开的口子" ✓。
//
// 它模拟真实网关的三件事 ✓：
//
//	① 面板下单  →  POST {本服务}/api/create-order            （面板调用 ✓）
//	② 用户支付  →  GET  {本服务}/pay?trade_no=...             （假收银台页面 ✓）
//	③ 回调入账  →  POST {面板}/api/v1/charge/callback/{site_id}（本服务调用 ✓，带 HMAC 签名 ✓）
//
// ⚠ 金额单位是 **分**（整数 ✓）：面板出站发 `amount`（分 ✓）+ `amount_yuan`（仅展示 ✓），
// 回调必须回传 `amount`（分 ✓）。
//
// ⚠ 收款有效期（-order-ttl ✓，默认 15 分钟 ✓）：真实网关的付款页会过期 ✓，
// 过期后**不再接收付款** ✓ —— 本服务用 `/expire?trade_no=`（测试开关 ✓）模拟这一刻 ✓。
//
// 用法：
//
//	cd tests/charge-e2e/mockchargesite && go run . -addr 127.0.0.1:9099 -secret cs_xxxx
//	# 打开 http://127.0.0.1:9099/orders 看订单 ✓，点"去支付"即是真实回调 ✓
//	# 或全自动：bash tests/charge-e2e/run.sh
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// order 是本服务记录的一笔"待支付订单" ✓（面板下单时带过来的信息 ✓）。
type order struct {
	TradeNo     string    `json:"trade_no"`
	SiteID      string    `json:"site_id"`      // 从 notify_url 里解析出来 ✓
	NotifyURL   string    `json:"notify_url"`   // 面板要求回调的地址 ✓
	AmountCents int64     `json:"amount_cents"` // ⚠ 单位：分（与面板一致 ✓）
	Currency    string    `json:"currency"`
	Subject     string    `json:"subject"`
	ClientEmail string    `json:"client_email"`
	PanelSign   string    `json:"panel_sign"`
	SignOK      *bool     `json:"sign_ok"`
	Status      string    `json:"status"` // pending / paid / failed / expired
	SiteTradeNo string    `json:"site_trade_no"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"` // 本服务的收款有效期 ✓（过期即不再接收付款 ✓）
	PaidAt      string    `json:"paid_at,omitempty"`
	LastResult  string    `json:"last_result"`
	LastCode    int       `json:"last_code"`
	BodyHash    string    `json:"body_hash,omitempty"`
	Sign        string    `json:"sign,omitempty"`
}

// Yuan 只用于页面展示 ✓。
func (o *order) Yuan() string { return fmt.Sprintf("%.2f", float64(o.AmountCents)/100) }

// Expired 判断本服务的收款窗口是否已过 ✓。
func (o *order) Expired() bool { return !o.ExpiresAt.IsZero() && time.Now().After(o.ExpiresAt) }

type server struct {
	mu       sync.Mutex
	orders   map[string]*order
	secrets  map[string]string // site_id -> callback secret ✓
	fallback string
	apiKey   string
	autopay  bool
	public   string
	orderTTL time.Duration
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9099", "监听地址")
	secret := flag.String("secret", "", "回调密钥（单站点）")
	sites := flag.String("sites", "", `多站点密钥，如 "3=cs_aaa,5=cs_bbb"`)
	apiKey := flag.String("apikey", "", "可选：校验面板下单签名用的 APIKey")
	autopay := flag.Bool("autopay", false, "打开收银台即自动支付")
	public := flag.String("public", "", "可选：payment_url 的外网前缀")
	ttl := flag.Duration("order-ttl", 15*time.Minute, "收款有效期（过期后不再接收付款）")
	flag.Parse()

	s := &server{
		orders: map[string]*order{}, secrets: map[string]string{},
		fallback: *secret, apiKey: *apiKey, autopay: *autopay,
		public: strings.TrimRight(*public, "/"), orderTTL: *ttl,
	}
	for _, part := range strings.Split(*sites, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			log.Fatalf("✗ -sites 格式应为 id=secret：%q", part)
		}
		s.secrets[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/create-order", s.handleCreateOrder)
	mux.HandleFunc("/pay", s.handlePayPage)
	mux.HandleFunc("/pay-api", s.handlePayAPI)
	mux.HandleFunc("/expire", s.handleExpire) // 测试开关：立刻把订单置为"收款窗口已过"
	mux.HandleFunc("/orders", s.handleOrders)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })

	if s.fallback == "" && len(s.secrets) == 0 {
		log.Printf("⚠ 没配 -secret/-sites → 支付时会因为没有回调密钥而失败 ✗")
	}
	log.Printf("模拟充值站点已启动 ✓ 监听 %s（收款有效期 %s ✓；订单页 http://%s/orders ✓）", *addr, *ttl, *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// ---------------------------------------------------------------------------
// ① 面板下单：POST /api/create-order
// ---------------------------------------------------------------------------

func (s *server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid json: " + err.Error()})
		return
	}
	tradeNo, _ := payload["trade_no"].(string)
	if tradeNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "trade_no required"})
		return
	}
	// ⚠ amount 是「分」（整数 ✓）：非整数一律报错，不静默取整 ✗
	cents, ok := jsonInt64(payload["amount"])
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": fmt.Sprintf("amount 必须是整数「分」✗（收到 %v）", payload["amount"])})
		return
	}
	currency, _ := payload["currency"].(string)
	subject, _ := payload["subject"].(string)
	clientEmail, _ := payload["client_email"].(string)
	notifyURL, _ := payload["notify_url"].(string)
	panelSign, _ := payload["sign"].(string)

	o := &order{
		TradeNo: tradeNo, NotifyURL: notifyURL, AmountCents: cents, Currency: currency,
		Subject: subject, ClientEmail: clientEmail, PanelSign: panelSign,
		SiteID: siteIDFromNotifyURL(notifyURL), Status: "pending",
		SiteTradeNo: "MOCK" + strconv.FormatInt(time.Now().UnixNano()%1e9, 10),
		CreatedAt:   time.Now(), ExpiresAt: time.Now().Add(s.orderTTL),
	}
	if s.apiKey != "" {
		ok := verifyPanelSign(payload, s.apiKey)
		o.SignOK = &ok
		if !ok {
			log.Printf("⚠ 面板下单签名校验失败 ✗ trade_no=%s（-apikey 与面板站点里的 APIKey 是否一致 ✓？）", tradeNo)
		}
	}

	s.mu.Lock()
	s.orders[tradeNo] = o
	s.mu.Unlock()

	log.Printf("① 面板下单 ✓ trade_no=%s amount=%d 分（%s 元）%s 收款有效期至 %s",
		tradeNo, cents, o.Yuan(), currency, o.ExpiresAt.Format("15:04:05"))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "payment_url": s.payURL(r, tradeNo),
		"expires_in": int(s.orderTTL.Seconds()), "error": "",
	})
}

// jsonInt64 把 JSON 数字安全地取成 int64 ✓（浮点带小数一律判非法 ✓）。
func jsonInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		if t != float64(int64(t)) {
			return 0, false
		}
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		return n, err == nil
	}
	return 0, false
}

// verifyPanelSign 复刻面板出站签名 ✓：删掉 sign 后 json.Marshal（map 字典序 ✓）再 HMAC-SHA256 ✓。
func verifyPanelSign(payload map[string]any, apiKey string) bool {
	clone := make(map[string]any, len(payload))
	for k, v := range payload {
		if k == "sign" {
			continue
		}
		clone[k] = v
	}
	body, _ := json.Marshal(clone)
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	got, _ := payload["sign"].(string)
	return hmac.Equal([]byte(want), []byte(got))
}

func (s *server) payURL(r *http.Request, tradeNo string) string {
	base := s.public
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/pay?trade_no=" + tradeNo
}

// ---------------------------------------------------------------------------
// ② 假收银台
// ---------------------------------------------------------------------------

func (s *server) handlePayPage(w http.ResponseWriter, r *http.Request) {
	tradeNo := r.URL.Query().Get("trade_no")
	s.mu.Lock()
	o := s.orders[tradeNo]
	s.mu.Unlock()
	if o == nil {
		http.Error(w, "订单不存在："+template.HTMLEscapeString(tradeNo), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = payPageTmpl.Execute(w, struct {
		O       *order
		Autopay bool
		Secret  bool
		Expired bool
	}{o, s.autopay, s.secretFor(o.SiteID) != "", o.Expired()})
}

// handleExpire 是**测试开关** ✓：立刻把某笔订单的收款窗口置为已过 ✓，
// 用来验证"过期后站点侧不再接收付款" ✓（真实网关是靠它自己的 TTL ✓）。
func (s *server) handleExpire(w http.ResponseWriter, r *http.Request) {
	tradeNo := r.URL.Query().Get("trade_no")
	s.mu.Lock()
	o := s.orders[tradeNo]
	if o != nil {
		o.ExpiresAt = time.Now().Add(-time.Minute)
		if o.Status == "pending" {
			o.Status = "expired"
		}
	}
	s.mu.Unlock()
	if o == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "order not found"})
		return
	}
	log.Printf("② 测试开关：订单收款窗口已置为过期 ✓ trade_no=%s", tradeNo)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "trade_no": tradeNo, "expires_at": o.ExpiresAt})
}

func (s *server) handlePayAPI(w http.ResponseWriter, r *http.Request) {
	tradeNo := r.URL.Query().Get("trade_no")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "paid"
	}
	code, body, err := s.pay(tradeNo, status)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "panel_code": code, "panel_body": body})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "panel_code": code, "panel_body": body})
}

// ---------------------------------------------------------------------------
// ③ 回调面板（真正入账的一步 ✓）
// ---------------------------------------------------------------------------

func (s *server) pay(tradeNo, status string) (int, string, error) {
	s.mu.Lock()
	o := s.orders[tradeNo]
	s.mu.Unlock()
	if o == nil {
		return 0, "", fmt.Errorf("订单不存在：%s", tradeNo)
	}
	// ⚠ 收款窗口已过 → **不再接收付款** ✓（真实网关同样会关掉付款页 ✓）
	if o.Expired() {
		return 0, "", fmt.Errorf("订单已过期，不再接收付款 ✗（expires_at=%s）", o.ExpiresAt.Format(time.RFC3339))
	}
	secret := s.secretFor(o.SiteID)
	if secret == "" {
		return 0, "", fmt.Errorf("站点 %s 没配回调密钥 ✗（用 -secret 或 -sites %s=cs_xxx ✓）", o.SiteID, o.SiteID)
	}

	paidAt := time.Now().UTC().Format(time.RFC3339)
	nonce := newNonce()
	ts := time.Now().Unix()

	// 回调 body：amount 用 **分**（int64 ✓，与面板订单同一个单位 ✓）
	body := map[string]any{
		"trade_no":      o.TradeNo,
		"site_trade_no": o.SiteTradeNo,
		"status":        status, // paid / failed（不进签名载荷 ✓）
		"amount":        o.AmountCents,
		"currency":      o.Currency,
		"timestamp":     ts,
		"nonce":         nonce,
		"paid_currency": o.Currency,
		"paid_amount":   o.AmountCents,
		"method":        "alipay",
		"exchange_rate": 1.0,
		"paid_at":       paidAt,
	}

	// 第 1 步：body_hash = SHA256(**加 sign 之前**的 body 字节) ✓ —— 与官方对接文档一致 ✓
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, "", err
	}
	bodyHash := sha256HexBytes(bodyBytes)

	// 第 2~4 步：8 字段载荷（字典序 ✓）→ HMAC-SHA256(回调密钥) ✓
	sigPayload := map[string]any{
		"amount":        o.AmountCents,
		"body_hash":     bodyHash,
		"currency":      o.Currency,
		"nonce":         nonce,
		"site_trade_no": o.SiteTradeNo,
		"timestamp":     ts,
		"trade_no":      o.TradeNo,
		"version":       "v1",
	}
	sigBytes, _ := json.Marshal(sigPayload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(sigBytes)
	sign := hex.EncodeToString(mac.Sum(nil))

	// 发送：把 sign 加进 body 再序列化 ✓（面板侧会剔除 sign 成员后重算 hash ✓，位置无关 ✓）
	body["sign"] = sign
	finalBody, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, o.NotifyURL, bytes.NewReader(finalBody))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("回调失败：%v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	s.mu.Lock()
	o.Status = status
	o.PaidAt = paidAt
	o.LastCode = resp.StatusCode
	o.LastResult = string(respBody)
	o.BodyHash = bodyHash
	o.Sign = sign
	s.mu.Unlock()

	log.Printf("③ 回调面板 ✓ trade_no=%s status=%s amount=%d 分 → HTTP %d %s",
		o.TradeNo, status, o.AmountCents, resp.StatusCode, strings.TrimSpace(string(respBody)))
	return resp.StatusCode, string(respBody), nil
}

func (s *server) secretFor(siteID string) string {
	if v, ok := s.secrets[siteID]; ok {
		return v
	}
	return s.fallback
}

func (s *server) handleOrders(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	list := make([]*order, 0, len(s.orders))
	for _, o := range s.orders {
		list = append(list, o)
	}
	s.mu.Unlock()
	if r.URL.Query().Get("format") == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = ordersPageTmpl.Execute(w, list)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

func siteIDFromNotifyURL(u string) string {
	parts := strings.Split(strings.TrimRight(u, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// newNonce 生成一次性随机串 ✓ —— 面板对 (site_id, nonce) 有唯一约束 ✓，
// 24h 内重复会被判重放 ✗，所以必须真随机 ✓。
func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

const payPageHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>模拟收银台</title>
<style>
 body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#f5f6f8;margin:0;padding:40px}
 .card{max-width:520px;margin:0 auto;background:#fff;border-radius:12px;padding:28px;box-shadow:0 2px 12px rgba(0,0,0,.08)}
 h1{font-size:18px;margin:0 0 16px}
 table{width:100%;border-collapse:collapse;font-size:14px}
 td{padding:6px 0;vertical-align:top} td:first-child{color:#888;width:110px}
 code{background:#f0f1f3;padding:2px 4px;border-radius:4px;font-size:12px;word-break:break-all}
 button{font-size:15px;padding:10px 18px;border-radius:8px;border:0;cursor:pointer;margin-right:10px}
 .pay{background:#07c160;color:#fff} .fail{background:#e0e0e0;color:#333}
 .res{margin-top:18px;font-size:13px;white-space:pre-wrap;word-break:break-all;background:#fafafa;border:1px solid #eee;border-radius:8px;padding:12px;display:none}
</style></head><body>
<div class="card">
  <h1>模拟充值站点 · 收银台</h1>
  <table>
    <tr><td>订单号</td><td><code>{{.O.TradeNo}}</code></td></tr>
    <tr><td>金额</td><td><b>{{.O.Yuan}} {{.O.Currency}}</b> <span style="color:#999">（{{.O.AmountCents}} 分）</span></td></tr>
    <tr><td>商品</td><td>{{.O.Subject}}</td></tr>
    <tr><td>收款有效期</td><td>{{.O.ExpiresAt.Format "2006-01-02 15:04:05"}} {{if .Expired}}<b style="color:#c00">（已过期，不再接收付款）</b>{{end}}</td></tr>
    <tr><td>站点 ID</td><td><code>{{.O.SiteID}}</code></td></tr>
    <tr><td>回调地址</td><td><code>{{.O.NotifyURL}}</code></td></tr>
  </table>
  {{if not .Secret}}<p style="color:#c00">⚠ 本服务未配置该站点的回调密钥 → 支付必然失败 ✗（用 -secret cs_xxx ✓）</p>{{end}}
  <div style="margin-top:20px">
    <button class="pay" onclick="doPay('paid')" {{if .Expired}}disabled{{end}}>支付成功</button>
    <button class="fail" onclick="doPay('failed')" {{if .Expired}}disabled{{end}}>支付失败</button>
  </div>
  <div class="res" id="res"></div>
</div>
<script>
async function doPay(status){
  const r = await fetch('/pay-api?trade_no={{.O.TradeNo}}&status=' + status, {method:'POST'});
  const j = await r.json();
  const el = document.getElementById('res');
  el.style.display = 'block';
  el.textContent = (j.ok ? '✓ 面板回调已受理\n' : '✗ 未受理\n') + JSON.stringify(j, null, 2);
}
{{if and .Autopay (not .Expired)}}window.addEventListener('load', () => doPay('paid'));{{end}}
</script>
</body></html>`

var payPageTmpl = template.Must(template.New("pay").Parse(payPageHTML))

const ordersPageHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>模拟充值站点 · 订单</title>
<style>
 body{font-family:system-ui,sans-serif;background:#f5f6f8;margin:0;padding:32px}
 h1{font-size:18px} table{width:100%;border-collapse:collapse;background:#fff;border-radius:10px;overflow:hidden;font-size:13px}
 th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #eee} th{background:#fafafa;color:#666}
 code{font-size:12px;word-break:break-all}
</style></head><body>
<h1>模拟充值站点 · 订单列表（共 {{len .}} 笔）</h1>
<table><tr><th>订单号</th><th>金额</th><th>状态</th><th>收款有效期</th><th>面板回调</th><th>操作</th></tr>
{{range .}}<tr>
  <td><code>{{.TradeNo}}</code></td>
  <td>{{.Yuan}} {{.Currency}}</td>
  <td>{{.Status}}</td>
  <td>{{.ExpiresAt.Format "15:04:05"}}</td>
  <td>{{if .LastCode}}HTTP {{.LastCode}}<br><code>{{.LastResult}}</code>{{else}}—{{end}}</td>
  <td><a href="/pay?trade_no={{.TradeNo}}">去支付</a></td>
</tr>{{end}}</table>
</body></html>`

var ordersPageTmpl = template.Must(template.New("orders").Parse(ordersPageHTML))
