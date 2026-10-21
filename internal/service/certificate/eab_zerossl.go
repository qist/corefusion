package certificate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/pkg/netutil"
)

// ZeroSSLEABEndpoint 是 ZeroSSL 用**邮箱换 EAB** 的接口 ✓。
//
// 机制与 acme.sh 完全一致 ✓（本机 `/root/.acme.sh/acme.sh` 源码可查 ✓）：
//
//	_ZERO_EAB_ENDPOINT="https://api.zerossl.com/acme/eab-credentials-email"
//	_eabresp=$(_post "email=$_email" $_ZERO_EAB_ENDPOINT)
//	_eab_id=... '"eab_kid"' ... / _eab_hmac_key=... '"eab_hmac_key"' ...
//
// 这正是"为什么用 acme.sh 时从没填过 EAB"的原因 ✓ ——
// ZeroSSL 的 ACME **强制要求** EAB ✗（实测卡在注册：`externalAccountRequired` ✓），
// 但它的 REST API 允许**用一个邮箱直接换一对** ✓，acme.sh 就自动替你换了并存起来 ✓。
const ZeroSSLEABEndpoint = "https://api.zerossl.com/acme/eab-credentials-email"

// zeroSSLEABResponse 是接口返回体 ✓（成功时含 eab_kid / eab_hmac_key ✓）。
type zeroSSLEABResponse struct {
	Success   bool   `json:"success"`
	EABKID    string `json:"eab_kid"`
	EABHMAC   string `json:"eab_hmac_key"`
	ErrorCode int    `json:"-"`
	ErrorType string `json:"-"`
	Raw       string `json:"-"`
	// 错误结构：{"success":false,"error":{"code":2900,"type":"missing_email"}} ✓
	Error *struct {
		Code int    `json:"code"`
		Type string `json:"type"`
		Info string `json:"info"`
	} `json:"error"`
}

// parseZeroSSLEABResponse 解析响应 ✓（单独拆出来是为了能脱网单测 ✓）。
func parseZeroSSLEABResponse(body []byte) (*zeroSSLEABResponse, error) {
	var r zeroSSLEABResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("unexpected response from ZeroSSL: %s", truncateForErr(string(body)))
	}
	r.Raw = truncateForErr(string(body))
	if r.Error != nil {
		r.ErrorCode = r.Error.Code
		r.ErrorType = r.Error.Type
	}
	if !r.Success {
		if r.ErrorType != "" {
			return &r, fmt.Errorf("ZeroSSL rejected the request: %s (code %d)", r.ErrorType, r.ErrorCode)
		}
		return &r, fmt.Errorf("ZeroSSL rejected the request: %s", r.Raw)
	}
	if strings.TrimSpace(r.EABKID) == "" || strings.TrimSpace(r.EABHMAC) == "" {
		return &r, fmt.Errorf("ZeroSSL returned no EAB credentials: %s", r.Raw)
	}
	return &r, nil
}

func truncateForErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// FetchZeroSSLEAB 用**邮箱**向 ZeroSSL 领取一对 EAB，并**存成一份 EAB 配置** ✓ ——
// 之后"按 CA 自动关联" ✓ 会让 ZeroSSL 也能零手工签发 ✓。
//
// ⚠ 说明：这等价于"用该邮箱在 ZeroSSL 取/建账号" ✓（acme.sh 默认行为 ✓）；
// 请使用你确实拥有的邮箱 ✓（ZeroSSL 可能对频率有限制 ✓）。
func (s *EABConfigService) FetchZeroSSLEAB(ctx context.Context, email string) (*entity.EABConfig, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	form := url.Values{}
	form.Set("email", email)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ZeroSSLEABEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := netutil.NewDefaultHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach ZeroSSL: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	parsed, perr := parseZeroSSLEABResponse(body)
	if perr != nil {
		return nil, perr
	}
	_ = parsed

	// 同一邮箱重复领取 → **就地更新**已有的那份 ✓（避免越点越多 ✗）
	name := "ZeroSSL - " + email
	list, lerr := s.repo.List(ctx)
	if lerr == nil {
		for i := range list {
			if strings.EqualFold(strings.TrimSpace(list[i].Name), name) {
				existing := list[i]
				existing.CAProvider = "zerossl"
				existing.KeyID = parsed.EABKID
				existing.MACKey = parsed.EABHMAC
				if err := s.repo.Update(ctx, &existing); err != nil {
					return nil, err
				}
				existing.HasMACKey = true
				return &existing, nil
			}
		}
	}

	return s.Create(ctx, EABConfigInput{
		Name:       name,
		CAProvider: "zerossl",
		KeyID:      parsed.EABKID,
		MACKey:     parsed.EABHMAC,
	})
}
