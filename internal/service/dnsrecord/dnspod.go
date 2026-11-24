package dnsrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultDNSPodAPIBase = "https://dnsapi.cn"

// dnspodUpdater 是 DNSPod（腾讯云旧版 dnsapi.cn）的解析写入适配器。
//
// 凭据：
//   - DNSPOD_API_KEY = "ID,Token"（控制台生成的 API Token，与证书通道一致）
//   - 或 DNSPOD_API_ID + DNSPOD_API_TOKEN
//
// 状态：已实现，未实测。新版腾讯云 DNSPod 建议用 tencentcloud 通道（未实现）。
//
// ⚠ DNSPod 没有「按记录名查 zone」的接口，这里用 Domain.List 拉全部域名取最长后缀。
type dnspodUpdater struct {
	loginToken string
	apiBase    string
	httpc      *http.Client
}

func newDNSPodUpdater(vars map[string]string, cfg ProviderConfig) (Updater, error) {
	token := firstVar(vars, "DNSPOD_API_KEY", "DNSPOD_TOKEN")
	if token == "" {
		id := firstVar(vars, "DNSPOD_API_ID")
		key := firstVar(vars, "DNSPOD_API_TOKEN")
		if id != "" && key != "" {
			token = id + "," + key
		}
	}
	if token == "" {
		return nil, fmt.Errorf("缺少 DNSPod 凭据（DNSPOD_API_KEY=\"ID,Token\" 或 DNSPOD_API_ID+DNSPOD_API_TOKEN）")
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 20 * time.Second}
	}
	base := strings.TrimRight(cfg.APIBase, "/")
	if base == "" {
		base = defaultDNSPodAPIBase
	}
	return &dnspodUpdater{loginToken: token, apiBase: base, httpc: httpc}, nil
}

func (u *dnspodUpdater) Provider() string { return "dnspod" }

func (u *dnspodUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, rec.Name)
	if err != nil {
		return err
	}
	sub := recordRR(rec.Name, zone)
	if sub == "" {
		return fmt.Errorf("%s 不是 %s 的子域名", rec.Name, zone)
	}
	records, err := u.listRecords(ctx, zone, sub, typ)
	if err != nil {
		return err
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 600
	}
	if len(records) > 0 {
		for _, r := range records[1:] {
			if err := u.removeRecord(ctx, zone, r.ID); err != nil {
				return fmt.Errorf("清理重复 %s 记录失败：%w", typ, err)
			}
		}
		form := url.Values{
			"domain":      {zone},
			"record_id":   {records[0].ID},
			"sub_domain":  {sub},
			"record_type": {typ},
			"record_line": {"默认"},
			"value":       {rec.Content},
			"ttl":         {strconv.Itoa(ttl)},
		}
		if err := u.post(ctx, "Record.Modify", form); err != nil {
			return fmt.Errorf("更新 DNSPod %s 记录 %s 失败：%w", typ, rec.Name, err)
		}
		return nil
	}
	form := url.Values{
		"domain":      {zone},
		"sub_domain":  {sub},
		"record_type": {typ},
		"record_line": {"默认"},
		"value":       {rec.Content},
		"ttl":         {strconv.Itoa(ttl)},
	}
	if err := u.post(ctx, "Record.Create", form); err != nil {
		return fmt.Errorf("创建 DNSPod %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *dnspodUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, name)
	if err != nil {
		return err
	}
	sub := recordRR(name, zone)
	if sub == "" {
		return nil
	}
	records, err := u.listRecords(ctx, zone, sub, typ)
	if err != nil {
		return err
	}
	for _, r := range records {
		if err := u.removeRecord(ctx, zone, r.ID); err != nil {
			return fmt.Errorf("删除 DNSPod %s 记录 %s 失败：%w", typ, name, err)
		}
	}
	return nil
}

type dnspodRecord struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// findZone 用 Domain.List 拉全部域名，取 fqdn 的最长后缀匹配。
func (u *dnspodUpdater) findZone(ctx context.Context, fqdn string) (string, error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	body, err := u.postRaw(ctx, "Domain.List", url.Values{})
	if err != nil {
		return "", fmt.Errorf("查询 DNSPod 域名列表失败：%w", err)
	}
	var resp struct {
		Domains []struct {
			Name string `json:"name"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析 DNSPod 域名列表失败：%w", err)
	}
	best := ""
	for _, d := range resp.Domains {
		name := strings.ToLower(strings.TrimSpace(d.Name))
		if name == "" {
			continue
		}
		if (fqdn == name || strings.HasSuffix(fqdn, "."+name)) && len(name) > len(best) {
			best = name
		}
	}
	if best == "" {
		return "", fmt.Errorf("凭据看不到 %s 所属的 DNSPod 域名（未授权或不在该账号）", fqdn)
	}
	return best, nil
}

func (u *dnspodUpdater) listRecords(ctx context.Context, zone, sub, typ string) ([]dnspodRecord, error) {
	form := url.Values{
		"domain":      {zone},
		"sub_domain":  {sub},
		"record_type": {typ},
	}
	body, err := u.postRaw(ctx, "Record.List", form)
	if err != nil {
		return nil, fmt.Errorf("查询 DNSPod %s 记录失败：%w", typ, err)
	}
	var resp struct {
		Records []dnspodRecord `json:"records"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 DNSPod %s 记录失败：%w", typ, err)
	}
	out := make([]dnspodRecord, 0, len(resp.Records))
	for _, r := range resp.Records {
		if strings.EqualFold(r.Name, sub) && strings.EqualFold(r.Type, typ) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (u *dnspodUpdater) removeRecord(ctx context.Context, zone, recordID string) error {
	if strings.TrimSpace(recordID) == "" {
		return nil
	}
	return u.post(ctx, "Record.Remove", url.Values{"domain": {zone}, "record_id": {recordID}})
}

func (u *dnspodUpdater) post(ctx context.Context, action string, form url.Values) error {
	_, err := u.postRaw(ctx, action, form)
	return err
}

// postRaw 调用 DNSPod 接口并校验 status.code == 1，返回原始 body。
func (u *dnspodUpdater) postRaw(ctx context.Context, action string, form url.Values) ([]byte, error) {
	if form == nil {
		form = url.Values{}
	}
	form.Set("login_token", u.loginToken)
	form.Set("format", "json")
	form.Set("lang", "cn")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.apiBase+"/"+action, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "corefusion-dnsrecord/1.0")
	resp, err := u.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(raw), 300))
	}
	var status struct {
		Status struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("DNSPod 返回非 JSON：%s", truncateStr(string(raw), 200))
	}
	if status.Status.Code != "1" {
		// 记录不存在等场景：Record.List 返回 code=10（无记录），对 Upsert 是正常空集。
		if action == "Record.List" && status.Status.Code == "10" {
			return []byte(`{"records":[]}`), nil
		}
		return nil, fmt.Errorf("DNSPod 错误 %s: %s", status.Status.Code, status.Status.Message)
	}
	return raw, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
