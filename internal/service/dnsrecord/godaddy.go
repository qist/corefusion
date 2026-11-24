package dnsrecord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultGoDaddyAPIBase = "https://api.godaddy.com"

// godaddyUpdater 是 GoDaddy 的解析写入适配器。
//
// 凭据：GODADDY_API_KEY + GODADDY_API_SECRET（与证书 DNS-01 通道一致）。
// 状态：已实现，未实测。
type godaddyUpdater struct {
	key     string
	secret  string
	apiBase string
	httpc   *http.Client
}

func newGoDaddyUpdater(vars map[string]string, cfg ProviderConfig) (Updater, error) {
	key := firstVar(vars, "GODADDY_API_KEY", "GODADDY_KEY")
	secret := firstVar(vars, "GODADDY_API_SECRET", "GODADDY_SECRET")
	if key == "" || secret == "" {
		return nil, fmt.Errorf("缺少 GoDaddy 凭据（GODADDY_API_KEY / GODADDY_API_SECRET）")
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 30 * time.Second}
	}
	base := strings.TrimRight(cfg.APIBase, "/")
	if base == "" {
		base = defaultGoDaddyAPIBase
	}
	return &godaddyUpdater{key: key, secret: secret, apiBase: base, httpc: httpc}, nil
}

func (u *godaddyUpdater) Provider() string { return "godaddy" }

type godaddyRecord struct {
	Data string `json:"data"`
	Name string `json:"name,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
	Type string `json:"type,omitempty"`
}

func (u *godaddyUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zone, rr, err := u.resolve(ctx, rec.Name, typ)
	if err != nil {
		return err
	}
	ttl := rec.TTL
	// GoDaddy 要求 TTL >= 600。
	if ttl < 600 {
		ttl = 600
	}
	body := []godaddyRecord{{Data: rec.Content, Name: rr, TTL: ttl, Type: typ}}
	if err := u.do(ctx, http.MethodPut, fmt.Sprintf("%s/v1/domains/%s/records/%s/%s", u.apiBase, zone, typ, rr), body); err != nil {
		return fmt.Errorf("写入 GoDaddy %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *godaddyUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zone, rr, err := u.resolve(ctx, name, typ)
	if err != nil {
		return err
	}
	if err := u.do(ctx, http.MethodDelete, fmt.Sprintf("%s/v1/domains/%s/records/%s/%s", u.apiBase, zone, typ, rr), nil); err != nil {
		return fmt.Errorf("删除 GoDaddy %s 记录 %s 失败：%w", typ, name, err)
	}
	return nil
}

// resolve 找到 fqdn 所属的已注册域名（逐级候选 GET /v1/domains/{candidate}），
// 返回 zone 与相对记录名（根用 @）。
func (u *godaddyUpdater) resolve(ctx context.Context, fqdn, typ string) (zone, rr string, err error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	for _, cand := range zoneCandidates(fqdn) {
		if err := u.do(ctx, http.MethodGet, fmt.Sprintf("%s/v1/domains/%s", u.apiBase, cand), nil); err == nil {
			zone = cand
			break
		}
	}
	if zone == "" {
		return "", "", fmt.Errorf("凭据看不到 %s 所属的 GoDaddy 域名（未授权或不在该账号）", fqdn)
	}
	rr = "@"
	if fqdn != zone {
		rr = strings.TrimSuffix(fqdn, "."+zone)
	}
	return zone, rr, nil
}

func (u *godaddyUpdater) do(ctx context.Context, method, url string, body any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "sso-key "+u.key+":"+u.secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}
	return nil
}
