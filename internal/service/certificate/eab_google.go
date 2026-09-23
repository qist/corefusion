package certificate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"

	"corefusion/internal/model/entity"
)

// GooglePublicCAKeyEndpoint 是 GCP 的**外部账号密钥（EAB）**创建接口 ✓。
//
//	POST https://publicca.googleapis.com/v1/projects/{PROJECT}/locations/global/externalAccountKeys
//	→ { "keyId": "...", "b64MacKey": "..." }   ← base64url（RFC 4648 §5 ✓）
//
// 官方流程（`gcloud publicca external-account-keys create` ✓ 等价 ✓）：
//  1. 启用 Public CA API：`gcloud services enable publicca.googleapis.com` ✓
//  2. 授予角色：`roles/publicca.externalAccountKeyCreator` ✓
//  3. 用**服务账号**（或用户凭据）换 access token ✓ → 调本接口 ✓
//
// ⚠ Google 与 ZeroSSL 不同 ✗：**邮箱换不到 EAB** ✗，必须走 GCP 授权 ✓
// （也就是说这条路**必须要授权** ✓，用户提供的服务账号 JSON 就是授权凭据 ✓）。
const GooglePublicCAKeyEndpoint = "https://publicca.googleapis.com/v1/projects/%s/locations/global/externalAccountKeys"

// googleCloudPlatformScope 是调 Public CA API 所需的最小作用域 ✓。
const googleCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// googleEABResponse 是创建接口的返回 ✓。
type googleEABResponse struct {
	KeyID     string `json:"keyId"`
	B64MacKey string `json:"b64MacKey"`
	// GCP 的错误结构 ✓
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// parseGoogleEABResponse 解析响应 ✓（脱网可测 ✓）。
func parseGoogleEABResponse(body []byte) (*googleEABResponse, error) {
	var r googleEABResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("unexpected response from Google Public CA: %s", truncateForErr(string(body)))
	}
	if r.Error != nil {
		hint := ""
		switch r.Error.Status {
		case "PERMISSION_DENIED":
			hint = "（需要给服务账号 roles/publicca.externalAccountKeyCreator ✓）"
		case "SERVICE_DISABLED":
			hint = "（需要先启用 publicca.googleapis.com ✓）"
		case "NOT_FOUND":
			hint = "（项目 ID 是否正确 ✓）"
		}
		return &r, fmt.Errorf("Google Public CA rejected: %s (code %d, %s)%s",
			r.Error.Message, r.Error.Code, r.Error.Status, hint)
	}
	if strings.TrimSpace(r.KeyID) == "" || strings.TrimSpace(r.B64MacKey) == "" {
		return &r, fmt.Errorf("Google returned no EAB key: %s", truncateForErr(string(body)))
	}
	return &r, nil
}

// googleAccessToken 用服务账号 JSON 换 access token ✓（authorized_user 类型也支持 ✓）。
func googleAccessToken(ctx context.Context, saJSON string) (string, error) {
	saJSON = strings.TrimSpace(saJSON)
	if saJSON == "" {
		return "", fmt.Errorf("service account JSON is required")
	}
	creds, err := google.CredentialsFromJSON(ctx, []byte(saJSON), googleCloudPlatformScope)
	if err != nil {
		return "", fmt.Errorf("invalid service account JSON: %w", err)
	}
	tok, err := creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("cannot obtain access token: %w", err)
	}
	return tok.AccessToken, nil
}

// FetchGoogleEAB 调 GCP 创建一对 EAB 并**存成/更新**一份 Google 的 EAB 配置 ✓。
//
// 两种授权方式（都算"授权" ✓）：
//   - serviceAccountJSON：**推荐** ✓ 服务账号密钥（可长期复用、面板可随时再申请 ✓）
//   - accessToken：临时令牌 ✓（`gcloud auth print-access-token` 的产物，约 1 小时 ✓，
//     适合手工试一次 ✓；不会入库 ✓）
//
// ⚠ 官方硬性规则 ✗（文档同款 ✓）：
//   - 一把 EAB **只能注册一个 ACME 账号** ✓，注册成功即失效 ✗；
//   - **7 天内不用就作废** ✗；
//   - 但用它注册出来的 **ACME 账号本身不过期** ✓ → 续期不需要 EAB ✓；
//   - **删掉 GCP 项目 → 该项目所有 ACME 账号立即失效** ✗ → 会无法续期 ✗。
func (s *EABConfigService) FetchGoogleEAB(ctx context.Context, projectID, serviceAccountJSON, accessToken string) (*entity.EABConfig, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	token := strings.TrimSpace(accessToken)
	if token == "" {
		t, err := googleAccessToken(ctx, serviceAccountJSON)
		if err != nil {
			return nil, err
		}
		token = t
	}

	url := fmt.Sprintf(GooglePublicCAKeyEndpoint, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Google Public CA: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	parsed, perr := parseGoogleEABResponse(body)
	if perr != nil {
		return nil, perr
	}

	name := "Google Public CA - " + projectID
	list, lerr := s.repo.List(ctx)
	if lerr == nil {
		for i := range list {
			if strings.EqualFold(strings.TrimSpace(list[i].Name), name) {
				existing := list[i]
				existing.CAProvider = "google"
				existing.KeyID = parsed.KeyID
				existing.MACKey = parsed.B64MacKey
				existing.GoogleProjectID = projectID
				if strings.TrimSpace(serviceAccountJSON) != "" {
					existing.GoogleSAJSON = strings.TrimSpace(serviceAccountJSON)
				}
				if err := s.repo.Update(ctx, &existing); err != nil {
					return nil, err
				}
				existing.HasMACKey = true
				existing.HasGoogleSA = strings.TrimSpace(existing.GoogleSAJSON) != ""
				return &existing, nil
			}
		}
	}

	return s.Create(ctx, EABConfigInput{
		Name:            name,
		CAProvider:      "google",
		KeyID:           parsed.KeyID,
		MACKey:          parsed.B64MacKey,
		GoogleProjectID: projectID,
		GoogleSAJSON:    strings.TrimSpace(serviceAccountJSON),
	})
}

// EnsureGoogleEAB 保证"Google 有一份**可直接用**的 EAB" ✓ ——
// 有就用 ✓；没有但存过服务账号 JSON → **现场再申请一把** ✓（这就是"首次申请即自动注册" ✓）。
//
// 为什么需要它 ✓：Google 的 EAB 是 **7 天 + 一次性** ✗，
// 一旦换成"新账号"（换数据目录 / 账号文件丢失 / 多节点各自注册 ✓）就需要新的 ✓；
// 存了服务账号就能自动续上 ✓，不用人工再去 GCP 点一次 ✓。
func (s *EABConfigService) EnsureGoogleEAB(ctx context.Context) (*entity.EABConfig, bool, error) {
	if cfg, err := s.GetByCA(ctx, "google"); err != nil {
		return nil, false, err
	} else if cfg != nil {
		return cfg, false, nil // 已有可用的 ✓
	}

	// 找一份"存过服务账号 JSON"的 Google 配置 ✓（哪怕它的 KID/MAC 已失效 ✗）
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, false, err
	}
	for i := range list {
		if !strings.EqualFold(normalizeCAProvider(list[i].CAProvider), "google") {
			continue
		}
		sa := strings.TrimSpace(list[i].GoogleSAJSON)
		if sa == "" {
			continue
		}
		proj := strings.TrimSpace(list[i].GoogleProjectID)
		if proj == "" {
			continue
		}
		cfg, ferr := s.FetchGoogleEAB(ctx, proj, sa, "")
		if ferr != nil {
			return nil, false, ferr
		}
		return cfg, true, nil // true = 本次是"新申请"的 ✓
	}
	return nil, false, nil // 没有可用的、也没存过授权 → 调用方给出清晰提示 ✓
}
