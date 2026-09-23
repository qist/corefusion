package certificate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
			hint = "（需要给服务账号 roles/publicca.externalAccountKeyCreator）"
		case "SERVICE_DISABLED":
			hint = "（需要先启用 publicca.googleapis.com）"
		case "NOT_FOUND":
			hint = "（请核对项目 ID）"
		}
		return &r, fmt.Errorf("Google Public CA rejected: %s (code %d, %s)%s",
			r.Error.Message, r.Error.Code, r.Error.Status, hint)
	}
	if strings.TrimSpace(r.KeyID) == "" || strings.TrimSpace(r.B64MacKey) == "" {
		return &r, fmt.Errorf("Google returned no EAB key: %s", truncateForErr(string(body)))
	}
	return &r, nil
}

// ErrGoogleNeedServiceAccountJSON 是"给错了凭据类型"的统一提示 ✓。
//
// ⚠ 这里最容易搞混的两种东西 ✗：
//   - **API Key**（`AIza…` 那串 ✓）：只是"公开标识" ✗，**不带身份** ✗ →
//     调 Public CA 的 externalAccountKeys.create 需要 IAM 权限 ✓，所以它**没用** ✗；
//   - **服务账号密钥**（GCP 后台"密钥 → 创建新密钥 → JSON"下载的**整个文件** ✓）：
//     它**本身就是"一把 key"** ✓（内含 client_email + private_key ✓）——
//     只是以 JSON 文件形式交付 ✓，面板拿它签 JWT 换 OAuth2 token ✓，再调 API ✓。
//
// 不想给这个文件也有两条路 ✓：① 用临时 access token（约 1 小时 ✓，不落库 ✓）；
// ② 干脆别用这里 —— 本地 `gcloud publicca external-account-keys create` 一次 ✓，
//
//	把打印出来的 keyId / b64MacKey **手填**到 EAB 配置的 EAB-KID / EAB-HMAC-Key ✓
//	（代价：Google 的 EAB 是 7 天 + 一次性 ✗ → 过期了要再点一次 ✓）。
const ErrGoogleNeedServiceAccountJSON = "这不是服务账号密钥 JSON。需要的是 GCP 里下载的那个密钥文件的完整内容" +
	"（JSON 文本，含 client_email / private_key）；API Key（AIza…）在这里没用。" +
	"若不想给文件：可改用临时 access token，或直接手填 EAB-KID / EAB-HMAC-Key。"

// normalizeServiceAccountJSON 容错 ✓：允许只粘"最小字段集"，
// 自动补上 oauth2 需要的 type / token_uri ✓（省得因为少两行就报错 ✓）。
func normalizeServiceAccountJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("请提供服务账号密钥文件内容，或临时 access token")
	}
	// 一眼看出不是 JSON：多半粘成了 API Key / 只粘了一行 ✓
	if !strings.HasPrefix(raw, "{") {
		if strings.HasPrefix(raw, "AIza") {
			return "", fmt.Errorf("这看起来是 Google API Key，不是服务账号密钥：%s", ErrGoogleNeedServiceAccountJSON)
		}
		return "", fmt.Errorf("服务账号密钥必须是那个密钥文件的完整内容（JSON 对象）：%s", ErrGoogleNeedServiceAccountJSON)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return "", fmt.Errorf("服务账号密钥不是合法 JSON（%v）：%s", err, ErrGoogleNeedServiceAccountJSON)
	}
	str := func(k string) string {
		if v, ok := m[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	if str("client_email") == "" && str("private_key") == "" {
		// authorized_user（gcloud 用户凭据 ✓）是另一套字段 ✓，交给 oauth2 自己判 ✓
		if str("type") != "authorized_user" {
			return "", fmt.Errorf("缺少 client_email / private_key：%s", ErrGoogleNeedServiceAccountJSON)
		}
	}
	if str("type") == "" {
		m["type"] = "service_account" // 最小集合时补上 ✓
	}
	if str("token_uri") == "" {
		m["token_uri"] = "https://oauth2.googleapis.com/token"
	}
	out, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// googleAccessToken 用服务账号 JSON 换 access token ✓（authorized_user 类型也支持 ✓）。
func googleAccessToken(ctx context.Context, saJSON string) (string, error) {
	normalized, err := normalizeServiceAccountJSON(saJSON)
	if err != nil {
		return "", err
	}
	creds, err := google.CredentialsFromJSON(ctx, []byte(normalized), googleCloudPlatformScope)
	if err != nil {
		return "", fmt.Errorf("这份服务账号密钥用不了（%v）：%s", err, ErrGoogleNeedServiceAccountJSON)
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
		// ⚠ 申请失败也**先把服务账号存下来** ✗→✓：否则用户填的内容白丢 ✗（真机反馈过 ✓），
		// 而且修好 IAM 后还能"自动再申请"，不用重新粘一遍 ✓。
		if _, serr := s.saveGoogleAuth(ctx, projectID, serviceAccountJSON); serr != nil {
			return nil, fmt.Errorf("%v（另外：保存服务账号失败 %v）", perr, serr)
		}
		log.Printf("[EAB] Google EAB create failed, but service account saved for retry: %v", perr)
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

// saveGoogleAuth 把 GCP 授权信息（项目 ID + 服务账号 JSON）落到一份 Google 配置里 ✓。
// 没有则新建一份（名称固定 ✓），有则就地更新 ✓；**不动已有的 KID/MAC** ✓。
func (s *EABConfigService) saveGoogleAuth(ctx context.Context, projectID, saJSON string) (*entity.EABConfig, error) {
	projectID = strings.TrimSpace(projectID)
	saJSON = strings.TrimSpace(saJSON)
	if projectID == "" && saJSON == "" {
		return nil, nil
	}
	name := "Google Public CA - " + projectID
	if projectID == "" {
		name = "Google Public CA"
	}
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if strings.EqualFold(normalizeCAProvider(list[i].CAProvider), "google") &&
			(strings.EqualFold(strings.TrimSpace(list[i].Name), name) || strings.TrimSpace(list[i].GoogleSAJSON) != "") {
			cfg := list[i]
			if projectID != "" {
				cfg.GoogleProjectID = projectID
			}
			if saJSON != "" {
				cfg.GoogleSAJSON = saJSON
			}
			if err := s.repo.Update(ctx, &cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		}
	}
	return s.Create(ctx, EABConfigInput{Name: name, CAProvider: "google", GoogleProjectID: projectID, GoogleSAJSON: saJSON})
}

// StoredGoogleAuth 返回已保存的 GCP 授权（项目 ID + 服务账号 JSON）✓。
// 没有就返回空串 ✓。用于"用户已经存过一次 → 之后不用再粘" ✓。
func (s *EABConfigService) StoredGoogleAuth(ctx context.Context) (projectID, saJSON string) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return "", ""
	}
	for i := range list {
		if !strings.EqualFold(normalizeCAProvider(list[i].CAProvider), "google") {
			continue
		}
		sa := strings.TrimSpace(list[i].GoogleSAJSON)
		if sa == "" {
			continue
		}
		return strings.TrimSpace(list[i].GoogleProjectID), sa
	}
	return "", ""
}

// MintGoogleEAB 用**已存的服务账号**申请一把**全新的** Google EAB ✓（并就地更新配置 ✓）。
//
// 为什么要"强制新的" ✗：Google 的 EAB **一次性** ✓ —— 已经用来注册过账号的那把**已作废** ✗。
// 而"某个节点第一次签发"必须要一把**没用过的** ✓（例如 1.8 这种从没注册过账号的机器 ✓），
// 所以节点侧签发时不能直接复用库里那把 ✗。
func (s *EABConfigService) MintGoogleEAB(ctx context.Context) (*entity.EABConfig, error) {
	projectID, saJSON := s.StoredGoogleAuth(ctx)
	if projectID == "" || saJSON == "" {
		return nil, fmt.Errorf("no stored GCP service account (set it in the Google EAB config first)")
	}
	return s.FetchGoogleEAB(ctx, projectID, saJSON, "")
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
