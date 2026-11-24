package dnsrecord

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultNamecheapAPIBase = "https://api.namecheap.com/xml.response"

// namecheapUpdater 是 Namecheap 的解析写入适配器。
//
// Namecheap 的 API **只有整组替换**（getHosts → setHosts），没有单条增删；
// 因此这里每次都读出全部记录、就地修改目标记录、再整体提交（其余记录原样保留）。
//
// 凭据：NAMECHEAP_API_USER + NAMECHEAP_API_KEY（与证书通道一致），
// 可选 NAMECHEAP_CLIENT_IP（API 要求白名单 IP；留空则自动探测公网 IP）。
// 状态：已实现，未实测。
type namecheapUpdater struct {
	apiUser  string
	apiKey   string
	clientIP string
	apiBase  string
	httpc    *http.Client
}

func newNamecheapUpdater(vars map[string]string, cfg ProviderConfig) (Updater, error) {
	user := firstVar(vars, "NAMECHEAP_API_USER", "NAMECHEAP_USER")
	key := firstVar(vars, "NAMECHEAP_API_KEY", "NAMECHEAP_KEY")
	if user == "" || key == "" {
		return nil, fmt.Errorf("缺少 Namecheap 凭据（NAMECHEAP_API_USER / NAMECHEAP_API_KEY）")
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 60 * time.Second}
	}
	base := strings.TrimSpace(cfg.APIBase)
	if base == "" {
		base = defaultNamecheapAPIBase
	}
	return &namecheapUpdater{
		apiUser:  user,
		apiKey:   key,
		clientIP: firstVar(vars, "NAMECHEAP_CLIENT_IP", "NAMECHEAP_IP"),
		apiBase:  base,
		httpc:    httpc,
	}, nil
}

func (u *namecheapUpdater) Provider() string { return "namecheap" }

type ncHost struct {
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	MXPref  string `xml:"MXPref,attr"`
	TTL     string `xml:"TTL,attr"`
}

func (u *namecheapUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zone, sub, err := u.resolve(ctx, rec.Name)
	if err != nil {
		return err
	}
	sld, tld := splitDomainTLD(zone)
	if sld == "" || tld == "" {
		return fmt.Errorf("无法拆分 Namecheap 域名 %s", zone)
	}
	hosts, err := u.getHosts(ctx, sld, tld)
	if err != nil {
		return err
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 1800
	}
	// 同名同类型收敛为一条：其余删除，第一条改成新值。
	kept := false
	out := make([]ncHost, 0, len(hosts)+1)
	for _, h := range hosts {
		if strings.EqualFold(h.Name, sub) && strings.EqualFold(h.Type, typ) {
			if kept {
				continue
			}
			kept = true
			h.Address = rec.Content
			h.TTL = strconv.Itoa(ttl)
			out = append(out, h)
			continue
		}
		out = append(out, h)
	}
	if !kept {
		out = append(out, ncHost{Name: sub, Type: typ, Address: rec.Content, TTL: strconv.Itoa(ttl), MXPref: "10"})
	}
	if err := u.setHosts(ctx, sld, tld, out); err != nil {
		return fmt.Errorf("写入 Namecheap %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *namecheapUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zone, sub, err := u.resolve(ctx, name)
	if err != nil {
		return err
	}
	sld, tld := splitDomainTLD(zone)
	hosts, err := u.getHosts(ctx, sld, tld)
	if err != nil {
		return err
	}
	out := make([]ncHost, 0, len(hosts))
	for _, h := range hosts {
		if strings.EqualFold(h.Name, sub) && strings.EqualFold(h.Type, typ) {
			continue
		}
		out = append(out, h)
	}
	if len(out) == len(hosts) {
		return nil // 本来就没有
	}
	if err := u.setHosts(ctx, sld, tld, out); err != nil {
		return fmt.Errorf("删除 Namecheap %s 记录 %s 失败：%w", typ, name, err)
	}
	return nil
}

// resolve 找出 fqdn 所属的注册域名，并返回相对记录名（根用 @）。
func (u *namecheapUpdater) resolve(ctx context.Context, fqdn string) (zone, sub string, err error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	for _, cand := range zoneCandidates(fqdn) {
		sld, tld := splitDomainTLD(cand)
		if sld == "" || tld == "" {
			continue
		}
		if _, err := u.getHosts(ctx, sld, tld); err == nil {
			zone = cand
			break
		}
	}
	if zone == "" {
		return "", "", fmt.Errorf("凭据看不到 %s 所属的 Namecheap 域名（未授权/未加白名单 IP）", fqdn)
	}
	sub = "@"
	if fqdn != zone {
		sub = strings.TrimSuffix(fqdn, "."+zone)
	}
	return zone, sub, nil
}

func (u *namecheapUpdater) getHosts(ctx context.Context, sld, tld string) ([]ncHost, error) {
	q := u.baseQuery("namecheap.domains.dns.getHosts")
	q.Set("SLD", sld)
	q.Set("TLD", tld)
	body, err := u.call(ctx, u.apiBase+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var resp struct {
		Hosts []ncHost `xml:"CommandResponse>DomainDNSGetHostsResult>host"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Namecheap 响应失败：%w", err)
	}
	return resp.Hosts, nil
}

func (u *namecheapUpdater) setHosts(ctx context.Context, sld, tld string, hosts []ncHost) error {
	form := u.baseQuery("namecheap.domains.dns.setHosts")
	form.Set("SLD", sld)
	form.Set("TLD", tld)
	for i, h := range hosts {
		idx := strconv.Itoa(i + 1)
		form.Set("HostName"+idx, h.Name)
		form.Set("RecordType"+idx, h.Type)
		form.Set("Address"+idx, h.Address)
		form.Set("TTL"+idx, orDefault(h.TTL, "1800"))
		if strings.EqualFold(h.Type, "MX") {
			form.Set("MXPref"+idx, orDefault(h.MXPref, "10"))
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.apiBase, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := u.do(req)
	if err != nil {
		return err
	}
	return checkNamecheapOK(body)
}

func (u *namecheapUpdater) call(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	body, err := u.do(req)
	if err != nil {
		return nil, err
	}
	if err := checkNamecheapOK(body); err != nil {
		return nil, err
	}
	return body, nil
}

func (u *namecheapUpdater) do(req *http.Request) ([]byte, error) {
	resp, err := u.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(body), 300))
	}
	return body, nil
}

func (u *namecheapUpdater) baseQuery(command string) url.Values {
	q := url.Values{
		"ApiUser":  {u.apiUser},
		"ApiKey":   {u.apiKey},
		"UserName": {u.apiUser},
		"Command":  {command},
		"ClientIp": {u.clientIP},
	}
	return q
}

// checkNamecheapOK 检查 <ApiResponse Status="OK">，否则把 Errors 抛出来。
func checkNamecheapOK(body []byte) error {
	var resp struct {
		Status string `xml:"Status,attr"`
		Errors []struct {
			Number  string `xml:"Number,attr"`
			Message string `xml:",chardata"`
		} `xml:"Errors>Error"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析 Namecheap 响应失败：%w", err)
	}
	if strings.EqualFold(resp.Status, "OK") {
		return nil
	}
	if len(resp.Errors) > 0 {
		return fmt.Errorf("Namecheap 错误 %s: %s", resp.Errors[0].Number, strings.TrimSpace(resp.Errors[0].Message))
	}
	return fmt.Errorf("Namecheap 返回非 OK 状态：%s", truncateStr(string(body), 200))
}

// splitDomainTLD 把 example.co.uk 拆成 ("example", "co.uk")。
// 规则无法覆盖全部后缀，这里用「最后一段作为 TLD，其余作为 SLD」保持简单；
// 两级后缀（co.uk / com.cn）由 zoneCandidates 逐级试探兜底。
func splitDomainTLD(domain string) (sld, tld string) {
	domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
	i := strings.Index(domain, ".")
	if i <= 0 || i == len(domain)-1 {
		return "", ""
	}
	// 两级公共后缀（co.uk、com.cn、com.au …）：把第一段并入 TLD。
	rest := domain[i+1:]
	if strings.Count(rest, ".") == 1 {
		switch rest {
		case "co.uk", "org.uk", "ac.uk", "gov.uk", "com.cn", "net.cn", "org.cn",
			"com.au", "net.au", "org.au", "co.jp", "co.kr", "com.br", "com.tw", "com.hk":
			return domain[:i], rest
		}
	}
	return domain[:i], rest
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
