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

const defaultCloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// cloudflareUpdater 是 Cloudflare 的解析写入适配器（已真机验证）。
type cloudflareUpdater struct {
	auth    cfAuth
	apiBase string
	httpc   *http.Client
}

func newCloudflareUpdater(vars map[string]string, cfg ProviderConfig) (Updater, error) {
	token := firstVar(vars, "CF_Token", "CF_DNS_API_TOKEN", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_DNS_API_TOKEN")
	email := firstVar(vars, "CF_API_EMAIL", "CLOUDFLARE_EMAIL")
	apiKey := firstVar(vars, "CF_API_KEY", "CLOUDFLARE_API_KEY")
	if token == "" && (email == "" || apiKey == "") {
		return nil, fmt.Errorf("缺少 Cloudflare 凭据（CF_Token 或 email+key）")
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 15 * time.Second}
	}
	base := strings.TrimRight(cfg.APIBase, "/")
	if base == "" {
		base = defaultCloudflareAPIBase
	}
	return &cloudflareUpdater{
		auth:    cfAuth{token: token, email: email, apiKey: apiKey},
		apiBase: base,
		httpc:   httpc,
	}, nil
}

func (u *cloudflareUpdater) Provider() string { return "cloudflare" }

func (u *cloudflareUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zoneID, _, err := u.findZone(ctx, rec.Name)
	if err != nil {
		return err
	}
	records, err := u.listRecords(ctx, zoneID, rec.Name, typ)
	if err != nil {
		return err
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 60
	}
	body := map[string]any{
		"type":    typ,
		"name":    rec.Name,
		"content": rec.Content,
		"ttl":     ttl,
	}
	// proxied 只对 A/AAAA/CNAME 有效；TXT 传它会报错。
	if typ == "A" || typ == "AAAA" || typ == "CNAME" {
		body["proxied"] = rec.Proxied
	}
	if len(records) > 0 {
		for _, r := range records[1:] {
			// 同名多条（历史脏数据）：只保留一条，其余删掉，避免客户端随机命中旧值。
			if _, err := u.cfDo(ctx, http.MethodDelete, fmt.Sprintf("%s/zones/%s/dns_records/%s", u.apiBase, zoneID, r.ID), nil); err != nil {
				return fmt.Errorf("清理重复 %s 记录 %s 失败：%w", typ, r.ID, err)
			}
		}
		if _, err := u.cfDo(ctx, http.MethodPut, fmt.Sprintf("%s/zones/%s/dns_records/%s", u.apiBase, zoneID, records[0].ID), body); err != nil {
			return fmt.Errorf("更新 %s 记录 %s → %s 失败：%w", typ, rec.Name, rec.Content, err)
		}
		return nil
	}
	if _, err := u.cfDo(ctx, http.MethodPost, fmt.Sprintf("%s/zones/%s/dns_records", u.apiBase, zoneID), body); err != nil {
		return fmt.Errorf("创建 %s 记录 %s → %s 失败：%w", typ, rec.Name, rec.Content, err)
	}
	return nil
}

func (u *cloudflareUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zoneID, _, err := u.findZone(ctx, name)
	if err != nil {
		return err
	}
	records, err := u.listRecords(ctx, zoneID, name, typ)
	if err != nil {
		return err
	}
	for _, r := range records {
		if _, err := u.cfDo(ctx, http.MethodDelete, fmt.Sprintf("%s/zones/%s/dns_records/%s", u.apiBase, zoneID, r.ID), nil); err != nil {
			return fmt.Errorf("删除 %s 记录 %s 失败：%w", typ, name, err)
		}
	}
	return nil
}

type cfResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result json.RawMessage `json:"result"`
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type cfAuth struct {
	token  string
	email  string
	apiKey string
}

func (u *cloudflareUpdater) cfDo(ctx context.Context, method, url string, body any) (json.RawMessage, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	if u.auth.token != "" {
		req.Header.Set("Authorization", "Bearer "+u.auth.token)
	} else {
		req.Header.Set("X-Auth-Email", u.auth.email)
		req.Header.Set("X-Auth-Key", u.auth.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var cr cfResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("Cloudflare 返回非 JSON（HTTP %d）", resp.StatusCode)
	}
	if !cr.Success {
		msg := "unknown error"
		if len(cr.Errors) > 0 {
			msg = fmt.Sprintf("%d %s", cr.Errors[0].Code, cr.Errors[0].Message)
		}
		return nil, fmt.Errorf("Cloudflare HTTP %d: %s", resp.StatusCode, msg)
	}
	return cr.Result, nil
}

// findZone 逐级向上找 fqdn 所属的 CF zone（支持子 zone，如 ugigc.dpdns.org）。
func (u *cloudflareUpdater) findZone(ctx context.Context, fqdn string) (string, string, error) {
	for _, candidate := range zoneCandidates(fqdn) {
		raw, err := u.cfDo(ctx, http.MethodGet,
			fmt.Sprintf("%s/zones?name=%s&per_page=5", u.apiBase, candidate), nil)
		if err != nil {
			continue
		}
		var zones []cfZone
		if err := json.Unmarshal(raw, &zones); err != nil {
			continue
		}
		for _, z := range zones {
			if strings.EqualFold(z.Name, candidate) {
				return z.ID, z.Name, nil
			}
		}
	}
	return "", "", fmt.Errorf("凭据看不到 %s 所属的 Cloudflare zone（未授权或域名不在该账号）", fqdn)
}

func (u *cloudflareUpdater) listRecords(ctx context.Context, zoneID, fqdn, typ string) ([]cfRecord, error) {
	raw, err := u.cfDo(ctx, http.MethodGet,
		fmt.Sprintf("%s/zones/%s/dns_records?type=%s&name=%s&per_page=50", u.apiBase, zoneID, typ, fqdn), nil)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 记录失败：%w", typ, err)
	}
	var recs []cfRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		return nil, fmt.Errorf("解析 %s 记录失败：%w", typ, err)
	}
	return recs, nil
}

// zoneCandidates 返回从最精确到最宽的 zone 候选：entry.a.b.com → a.b.com → b.com。
func zoneCandidates(fqdn string) []string {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(fqdn, ".")), ".")
	if len(parts) < 2 {
		return nil
	}
	var out []string
	for i := 1; i <= len(parts)-2; i++ {
		out = append(out, strings.Join(parts[i:], "."))
	}
	return out
}
