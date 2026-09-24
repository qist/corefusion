package certificates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/service/audit"
	certService "corefusion/internal/service/certificate"
)

// CertUpdateCallback is called after a certificate is updated or renewed.
// It allows external code (e.g. router wiring) to trigger distribution to nodes.
type CertUpdateCallback func(ctx context.Context, certID int64)

// Handler handles certificate HTTP requests.
// NodeCertIssuer 是"让某个节点在本机签发公网证书"的能力（由 NodeServerManager 实现 ✓）。
type NodeCertIssuer interface {
	IssuePublicCert(ctx context.Context, serverID int64, cmd *agentpb.IssuePublicCertCommand) error
	IsOnlineForServer(serverID int64) bool
}

// DNSChannelResolver 把"DNS 通道 ID"解析成 dns-01 提供者配置 ✓
// （由 `certService.DNSChannelService` 实现 ✓）。
type DNSChannelResolver interface {
	ResolveDNSChannel(ctx context.Context, id int64) (*cert.DNSProviderConfig, error)
}

// ACMEAccountInfo 是"签发时从所选账号取用的信息" ✓。
// CA/EAB 存在账号里 ✓ → 签发时自动带上 ✓，用户不用每次手填 ✗。
type ACMEAccountInfo struct {
	Email      string
	CAProvider string
	EABKeyID   string
	EABMACKey  string
}

// caNeedsEAB 判断某个 CA 是否**强制要求** EAB ✓ —— Google Public CA ✓、ZeroSSL ✓
// （真机验证过：ZeroSSL 报 externalAccountRequired ✓、Google 报 Accounts must have at least one contact ✓）。
// Let's Encrypt **不需要** EAB ✓（传了也无害 ✓）。
func caNeedsEAB(caProvider string) bool {
	switch strings.ToLower(strings.TrimSpace(caProvider)) {
	case "google", "zerossl":
		return true
	default:
		return false
	}
}

// resolveIssueKeyType 决定下发给节点的**密钥规格** ✓（key_type + key_bits ✓）。
//
// 优先级：**请求显式指定 > 库记录兜底 > 空**（空/0 = 让节点用历史默认 EC256 ✓）。
//
// ⚠ 库记录兜底是关键 ✗：续签已有证书时前端往往不传密钥类型 ✓，
// 若直接留空 → 节点侧 `resolveKeyType("",0)` 回退 **EC256** ✗
// → 库里登记 `rsa/4096` 的证书被"续"成 EC 证书 ✗
// （真机现象：记录写 rsa、落地文件却是 `EC PRIVATE KEY` ✓）。
func resolveIssueKeyType(reqType string, reqBits int, dbType string, dbBits int) (string, int) {
	keyType := strings.TrimSpace(reqType)
	if keyType == "" {
		keyType = strings.TrimSpace(dbType)
	}
	keyBits := reqBits
	if keyBits <= 0 {
		keyBits = dbBits
	}
	if keyBits < 0 {
		keyBits = 0 // 防御：库里脏数据不该变成负数下发 ✗
	}
	return keyType, keyBits
}

// EABConfigInfo 是"从独立 EAB 配置里取到的内容" ✓。
type EABConfigInfo struct {
	Name       string
	CAProvider string
	KeyID      string
	MACKey     string
}

// EABConfigResolver 解析**独立**的 EAB 配置 ✓（和 DNS 通道一样的独立配置项 ✓）——
// 签发时只选关联 ✓，不用手填 EAB-KID / EAB-HMAC-Key ✗。
type EABConfigResolver interface {
	ResolveEABConfig(ctx context.Context, id int64) (*EABConfigInfo, error)
	// ResolveEABConfigForCA 按 CA **自动关联**一份 EAB ✓（Google / ZeroSSL 用 ✓）。
	// 返回 (nil, nil) 表示该 CA 没配过 EAB ✓（不当作错误 ✗，由调用方决定怎么提示 ✓）。
	ResolveEABConfigForCA(ctx context.Context, caProvider string) (*EABConfigInfo, error)
	// FetchZeroSSLEAB 用邮箱向 ZeroSSL 自动领取 EAB ✓ —— 首次申请即"自动注册" ✓
	//（ZeroSSL 强制要求 EAB ✗，但它的 REST API 允许用邮箱直接换一对 ✓，acme.sh 同机制 ✓）
	FetchZeroSSLEAB(ctx context.Context, email string) (*EABConfigInfo, error)
	// FetchGoogleEAB 用 **GCP 授权**创建一对 Google EAB ✓ ——
	// ⚠ Google 不像 ZeroSSL 能用邮箱换 ✗，**必须授权** ✓（服务账号 JSON 或临时 access token ✓）。
	FetchGoogleEAB(ctx context.Context, projectID, serviceAccountJSON, accessToken string) (*EABConfigInfo, error)
	// EnsureGoogleEAB 保证 Google 有一份**可用** EAB ✓：缺/过期就用存过的服务账号**现场再申请一把** ✓
	//（Google 的 EAB 是 7 天 + 一次性 ✗，这条让"首次申请即自动注册"在 Google 上也能成立 ✓）。
	EnsureGoogleEAB(ctx context.Context) (*EABConfigInfo, bool, error)
	// MintGoogleEAB **强制**用已存服务账号申请一把**全新**的 Google EAB ✓ ——
	// 给"节点首次注册"用 ✓：Google 的 EAB 一次性 ✓，库里那把多半已被注册用掉、已作废 ✗。
	MintGoogleEAB(ctx context.Context) (*EABConfigInfo, error)
}

// ACMEEmailResolver 解析 ACME 账号（由面板的 ACME 邮箱/账号服务实现 ✓）。
type ACMEEmailResolver interface {
	ResolveACMEEmail(ctx context.Context, id int64) (string, error)
	// ResolveACMEAccount 返回账号的邮箱 + CA + EAB ✓（id<=0 表示默认账号 ✓）。
	ResolveACMEAccount(ctx context.Context, id int64) (ACMEAccountInfo, error)
}

type Handler struct {
	certSvc       *certService.Service
	onCertUpdated CertUpdateCallback
	nodeIssuer    NodeCertIssuer
	acmeEmails    ACMEEmailResolver
	dnsChannels   DNSChannelResolver
	eabConfigs    EABConfigResolver
}

// SetACMEEmailResolver 注入 ACME 账号解析器 ✓（邮箱归面板 ✓，节点不配置 ✗）。
func (h *Handler) SetACMEEmailResolver(r ACMEEmailResolver) { h.acmeEmails = r }

// SetEABConfigResolver 注入"独立 EAB 配置"的解析能力 ✓。
func (h *Handler) SetEABConfigResolver(r EABConfigResolver) { h.eabConfigs = r }

// SetDNSChannelResolver 注入 DNS 通道解析器 ✓（凭据存在通道里 ✓，签发时按 ID 取 ✓）。
func (h *Handler) SetDNSChannelResolver(r DNSChannelResolver) { h.dnsChannels = r }

// SetNodeIssuer 注入"节点本机签发"能力 ✓（面板在 SNAT 后面时，公网证书只能让节点自己签 ✓）。
func (h *Handler) SetNodeIssuer(i NodeCertIssuer) { h.nodeIssuer = i }

// NewHandler creates a new certificate handler.
func NewHandler(certSvc *certService.Service) *Handler {
	return &Handler{certSvc: certSvc}
}

// OnCertUpdated sets the callback invoked after a certificate is updated/renewed.
func (h *Handler) OnCertUpdated(cb CertUpdateCallback) {
	h.onCertUpdated = cb
}

// fireCertUpdated asynchronously calls the update callback if set.
func (h *Handler) fireCertUpdated(certID int64) {
	if h.onCertUpdated == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		h.onCertUpdated(ctx, certID)
	}()
}

// stripDataPrefix removes "data/" prefix from paths for relative display.
func stripDataPrefix(certs []entity.Certificate) {
	for i := range certs {
		certs[i].CertPath = strings.TrimPrefix(certs[i].CertPath, "data/")
		certs[i].KeyPath = strings.TrimPrefix(certs[i].KeyPath, "data/")
		certs[i].HasContent = strings.TrimSpace(certs[i].CertContent) != "" &&
			strings.TrimSpace(certs[i].KeyContent) != ""
		// ⚠ 接口出参里**必须清掉 PEM/私钥** ✗：以前列表/详情会把 `key_content`（私钥！✗）
		// 原样回给前端 ✓ —— 前端从来不用它 ✓（只在**上传表单**里由用户填 ✓），
		// 需要内容的操作（下载）在**服务端**读库 ✓。所以清空是净收益 ✓。
		certs[i].CertContent = ""
		certs[i].KeyContent = ""
	}
}

// List handles GET /api/v1/certificates.
func (h *Handler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	// ⚠ 面板 Web **全部显示** ✓（用户口径 ✓："面板当然要全部显示" ✓）——
	// **不隐藏**任何证书 ✗（哪怕它是只有 IP 的证书 ✓）。
	// "不落地"这个词指的是**不写磁盘文件** ✗，不是"不入库"✗、更不是"不显示"✗。
	certs, total, err := h.certSvc.List(c.Request.Context(), q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_CERTIFICATES_FAILED", err.Error())
		return
	}

	stripDataPrefix(certs)
	response.Paginated(c, certs, total, q.Limit, q.Offset)
}

// Create handles POST /api/v1/certificates.
func (h *Handler) Create(c *gin.Context) {
	var input certService.CreateCertificateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	cert, err := h.certSvc.Create(c.Request.Context(), input)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_create", input.Domain, false, err.Error())
		response.InternalError(c, "CREATE_CERTIFICATE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_create", fmt.Sprintf("created cert: %s", cert.Domain), true, "")

	response.Created(c, cert)
}

// GetByID handles GET /api/v1/certificates/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}

	cert, err := h.certSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "CERTIFICATE_NOT_FOUND", "certificate not found")
		return
	}

	stripDataPrefix([]entity.Certificate{*cert})
	response.OK(c, cert)
}

// Update handles PUT /api/v1/certificates/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}

	var input certService.UpdateCertificateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	cert, err := h.certSvc.Update(c.Request.Context(), id, input)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_update", fmt.Sprintf("cert %d", id), false, err.Error())
		response.InternalError(c, "UPDATE_CERTIFICATE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_update", fmt.Sprintf("updated cert %d: %s", id, cert.Domain), true, "")

	h.fireCertUpdated(id)
	response.OK(c, cert)
}

// Delete handles DELETE /api/v1/certificates/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}

	if err := h.certSvc.Delete(c.Request.Context(), id); err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_delete", fmt.Sprintf("cert %d", id), false, err.Error())
		response.InternalError(c, "DELETE_CERTIFICATE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_delete", fmt.Sprintf("deleted cert %d", id), true, "")

	response.OK(c, gin.H{"message": "certificate deleted"})
}

// IssueOnNode handles POST /api/v1/certificates/issue-on-node.
//
// 面板要能**选节点**签发 ✓：面板常位于 SNAT 后面（CA 回连不到面板的 80 端口 ✗），
// 而节点有公网 IP 与 80 入站 ✓ → 由选定节点在本机完成 ACME 验证与签发 ✓，
// 签完把 PEM 回传（AgentReport.PUBLIC_CERT_ISSUED ✓）→ 面板落库并绑定该节点 ✓。
// 注意：这是**异步**的（节点签发需要时间 ✓），接口立刻返回 202 ✓。
func (h *Handler) IssueOnNode(c *gin.Context) {
	var req struct {
		NodeID        int64    `json:"node_id" binding:"required"`
		CertID        int64    `json:"cert_id"` // 续签已有证书时带上（可选 ✓）
		Domains       []string `json:"domains"` // 新签发时必填 ✓
		ChallengeType string   `json:"challenge_type"`
		CAProvider    string   `json:"ca_provider"`
		Profile       string   `json:"profile"`
		DNSChannelID  int64    `json:"dns_channel_id"` // dns-01 时用哪个 DNS 通道 ✓
		Webroot       string   `json:"webroot"`        // 留空则由节点按面板生成的 web 配置处理挑战 ✓
		Email         string   `json:"email"`          // 一般留空：由面板的 ACME 账号解析 ✓
		EmailID       int64    `json:"email_id"`       // 指定用哪个 ACME 账号（0 = 默认 ✓）
		// ⚠ 前端一直发的是**扁平**的这两个字段 ✓，而后端此前只认 `eab:{key_id,mac_key}` ✗
		// → 手填的 EAB 根本传不进来 ✗（Google / ZeroSSL 必填 EAB ✗ → 直接签不了 ✗）。
		EABKeyID  string `json:"eab_key_id"`
		EABMACKey string `json:"eab_mac_key"`
		// ⚠ 密钥类型必须能传 ✗：面板记录里选了 rsa/4096 ✓，指令里不带的话节点只能签出 EC256 ✗
		// （真机现象：记录 rsa、落地却是 `EC PRIVATE KEY` ✓）。留空则用 cert_id 对应记录的 ✓。
		KeyType string `json:"key_type"`
		KeyBits int    `json:"key_bits"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if h.nodeIssuer == nil {
		response.InternalError(c, "NOT_SUPPORTED", "node-side certificate issuance is not available")
		return
	}

	identifiers := make([]string, 0, len(req.Domains))
	for _, d := range req.Domains {
		if d = strings.TrimSpace(d); d != "" {
			identifiers = append(identifiers, d)
		}
	}
	// 续签已有证书：标识符 + **密钥类型**都从库里取 ✓（避免前端重复填、也避免填错/丢参数 ✗）
	//
	// ⚠ 密钥类型必须兜底 ✗：前端不传、指令里也没有 → 节点侧退回 EC256 ✗，
	// 于是"库里 rsa 的证书被续成 EC 证书" ✗（真机踩到 ✓）。
	// 所以这里**只要带 cert_id 就读库**（即使 domains 已显式给了 ✓）。
	var existingKeyType string
	var existingKeyBits int
	if req.CertID > 0 {
		existing, err := h.certSvc.GetByID(c.Request.Context(), req.CertID)
		if err != nil || existing == nil {
			// 显式给了 domains（新签发场景 ✓）就容错 ✓；没给则必须报错 ✗（与原来口径一致 ✓）
			if len(identifiers) == 0 {
				response.BadRequest(c, "CERT_NOT_FOUND", "certificate not found")
				return
			}
		} else {
			existingKeyType, existingKeyBits = existing.KeyType, existing.KeyBits
			if len(identifiers) == 0 {
				if existing.Domains != "" {
					var arr []string
					if json.Unmarshal([]byte(existing.Domains), &arr) == nil {
						for _, d := range arr {
							if d = strings.TrimSpace(d); d != "" {
								identifiers = append(identifiers, d)
							}
						}
					}
				}
				if len(identifiers) == 0 && strings.TrimSpace(existing.Domain) != "" {
					identifiers = append(identifiers, strings.TrimSpace(existing.Domain))
				}
			}
		}
	}
	if len(identifiers) == 0 {
		response.BadRequest(c, "INVALID_REQUEST", "domains or cert_id is required")
		return
	}

	// 节点必须在线，否则指令发不出去（离线节点会在重连时由续期调度重新下发 ✓）
	if !h.nodeIssuer.IsOnlineForServer(req.NodeID) {
		response.BadRequest(c, "NODE_OFFLINE", "selected node is offline; the agent must be online to issue on it")
		return
	}

	// 邮箱/CA/EAB 都来自**面板的 ACME 账号** ✓（用户选了就用选的，没选用默认 ✓）——
	// 节点侧不需要任何 acme 配置 ✗。
	email := strings.TrimSpace(req.Email)
	caProvider := strings.TrimSpace(req.CAProvider)
	eabKID := strings.TrimSpace(req.EABKeyID)
	eabMAC := strings.TrimSpace(req.EABMACKey)
	if h.acmeEmails != nil && (email == "" || caProvider == "" || eabMAC == "") {
		// 账号里存过 EAB（ZeroSSL / Google ✓）→ 自动补上 ✓，用户不用每次手填 ✗
		if info, rerr := h.acmeEmails.ResolveACMEAccount(c.Request.Context(), req.EmailID); rerr == nil {
			if email == "" {
				email = strings.TrimSpace(info.Email)
			}
			if caProvider == "" {
				caProvider = strings.TrimSpace(info.CAProvider)
			}
			if eabKID == "" {
				eabKID = strings.TrimSpace(info.EABKeyID)
			}
			if eabMAC == "" {
				eabMAC = strings.TrimSpace(info.EABMACKey)
			}
		}
	}
	if strings.TrimSpace(email) == "" {
		if h.acmeEmails == nil {
			// 快速失败 ✓：否则签发的指令会缺邮箱 ✗，只在节点侧报错 ✗（真机踩过 ✓）
			response.InternalError(c, "NO_ACME_RESOLVER", "ACME account resolver is not configured on the panel")
			return
		}
		response.BadRequest(c, "NO_ACME_ACCOUNT", "no ACME account email available on the panel")
		return
	}

	if strings.TrimSpace(caProvider) == "" {
		caProvider = strings.TrimSpace(req.CAProvider)
	}
	// ⚠ 必须把 EAB 一起下发 ✗ —— 目标节点可能是**从没注册过账号**的机器（例如 1.8 ✓），
	// 它第一次签发要真的去注册 ✓，而 Google/ZeroSSL 注册**必须要 EAB** ✗。
	// 库里那把很可能已被别的账号用掉（Google 的 EAB 是一次性 ✗）→ **现场申请一把新的** ✓。
	if eabMAC == "" && caNeedsEAB(caProvider) && h.eabConfigs != nil {
		switch strings.ToLower(strings.TrimSpace(caProvider)) {
		case "google":
			if minted, merr := h.eabConfigs.MintGoogleEAB(c.Request.Context()); merr == nil && minted != nil && minted.KeyID != "" {
				eabKID, eabMAC = minted.KeyID, minted.MACKey
				log.Printf("[cert] node issuance: minted fresh Google EAB kid=%s for node %d", eabKID, req.NodeID)
			} else if merr != nil {
				log.Printf("[cert] node issuance: mint Google EAB failed: %v", merr)
			}
		case "zerossl":
			if strings.Contains(email, "@") {
				if fetched, ferr := h.eabConfigs.FetchZeroSSLEAB(c.Request.Context(), email); ferr == nil && fetched != nil && fetched.KeyID != "" {
					eabKID, eabMAC = fetched.KeyID, fetched.MACKey
					log.Printf("[cert] node issuance: fetched fresh ZeroSSL EAB kid=%s for node %d", eabKID, req.NodeID)
				} else if ferr != nil {
					log.Printf("[cert] node issuance: fetch ZeroSSL EAB failed: %v", ferr)
				}
			}
		}
		if eabKID == "" || eabMAC == "" {
			// 现场申请不到（没授权/没邮箱 ✓）→ 用库里那把兜底 ✓（对已有账号的节点本来就够用 ✓）
			if cfg, cerr := h.eabConfigs.ResolveEABConfigForCA(c.Request.Context(), caProvider); cerr == nil && cfg != nil {
				eabKID, eabMAC = cfg.KeyID, cfg.MACKey
				log.Printf("[cert] node issuance: using stored %s EAB kid=%s", caProvider, eabKID)
			}
		}
	}

	// 密钥类型：请求里显式指定的优先 ✓，否则用**库记录**兜底 ✓；
	// 两个都为空 → 保持空串 ✓，节点侧回退历史默认 EC256 ✓（与"不指定"的旧行为一致 ✓）。
	keyType, keyBits := resolveIssueKeyType(req.KeyType, req.KeyBits, existingKeyType, existingKeyBits)

	cmd := &agentpb.IssuePublicCertCommand{
		CertId:      req.CertID,
		Identifiers: identifiers,
		Challenge:   req.ChallengeType,
		Ca:          caProvider,
		Profile:     req.Profile,
		Webroot:     req.Webroot,
		Email:       email,
		// ⚠ 这两行原来漏了 ✗ → 节点侧拿不到 EAB → Google/ZeroSSL 首次注册必失败 ✗
		EabKeyId:  eabKID,
		EabMacKey: eabMAC,
		// ⚠ 这两行原来**根本没有** ✗ → 节点只能签 EC256 ✗（见请求结构体上的注释 ✓）
		KeyType: keyType,
		KeyBits: int32(keyBits),
		// ⚠ 面板通道 id 必须随指令下发 ✗：节点签发用不到 ✓，
		// 但**回传时要原样带回** ✓ → 面板落库才能记住"创建时选的是哪条通道" ✓
		//（不然 dns_channel_id 永远是 0 ✗ → 自动续期按 dns-01 找不到通道 → 跳过 ✗）。
		DnsChannelId: req.DNSChannelID,
	}
	log.Printf("[cert] issue-on-node: node=%d ca=%q eab=%v email=%q key_type=%q key_bits=%d",
		req.NodeID, caProvider, eabKID != "" && eabMAC != "", email, keyType, keyBits)
	// dns-01：把面板里存的 DNS 通道（提供者 + 凭据 ✓）随指令下发给节点 ✓
	if strings.EqualFold(strings.TrimSpace(req.ChallengeType), "dns-01") && req.DNSChannelID > 0 {
		if h.dnsChannels == nil {
			response.InternalError(c, "NO_DNS_CHANNEL_RESOLVER", "DNS channel resolver is not configured on the panel")
			return
		}
		dnsCfg, derr := h.dnsChannels.ResolveDNSChannel(c.Request.Context(), req.DNSChannelID)
		if derr != nil {
			response.BadRequest(c, "DNS_CHANNEL_NOT_FOUND", derr.Error())
			return
		}
		if dnsCfg != nil {
			cmd.DnsProvider = string(dnsCfg.Provider)
			cmd.DnsEnvVars = dnsCfg.EnvVars
		}
	}
	if err := h.nodeIssuer.IssuePublicCert(c.Request.Context(), req.NodeID, cmd); err != nil {
		audit.RecordNodeFromCtx(c, req.NodeID, "cert_issue_on_node", strings.Join(identifiers, ","), false, err.Error())
		response.InternalError(c, "ISSUE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, req.NodeID, "cert_issue_on_node", strings.Join(identifiers, ","), true, "")
	response.OK(c, gin.H{
		"message":     "证书签发指令已下发到节点，签完后会自动入库并绑定该节点",
		"node_id":     req.NodeID,
		"identifiers": identifiers,
	})
}

// Issue handles POST /api/v1/certificates/issue.
func (h *Handler) Issue(c *gin.Context) {
	var req struct {
		Domains       []string `json:"domains" binding:"required"`
		ChallengeType string   `json:"challenge_type"`
		// DNSChannelID 是面板里存的"DNS 通道"（提供者 + 凭据 ✓）。
		// ⚠ 以前这里**完全没解析** ✗：前端一直在传 `dns_channel_id` ✓，后端却把它丢掉 ✗ →
		// dns-01 走到最后只会报 `dns provider config required` ✗（等于 DNS 通道从来没生效过 ✗）。
		DNSChannelID int64  `json:"dns_channel_id"`
		CAProvider   string `json:"ca_provider"`
		Profile      string `json:"profile"`
		DNSProvider  *struct {
			Provider string            `json:"provider"`
			EnvVars  map[string]string `json:"env_vars"`
		} `json:"dns_provider,omitempty"`
		EAB *struct {
			KeyID  string `json:"key_id"`
			MACKey string `json:"mac_key"`
		} `json:"eab,omitempty"`
		// ⚠ 前端实际发的是**扁平**字段 ✓，此前后端只认 `eab:{key_id,mac_key}` ✗
		// → 手填的 EAB 根本进不来 ✗（Google / ZeroSSL 必填 EAB ✗ → 直接签不了 ✗）。
		EABKeyID  string `json:"eab_key_id"`
		EABMACKey string `json:"eab_mac_key"`
		// EmailID 指定用哪个 ACME 账号（0 = 默认 ✓）→ 账号里存的 CA/EAB 自动生效 ✓
		EmailID int64 `json:"email_id"`
		// EABConfigID 指定用哪份**独立 EAB 配置** ✓（Google / ZeroSSL 用 ✓）——
		// 签发弹窗里**只选关联** ✓，不用手填 EAB-KID / EAB-HMAC-Key ✗。
		EABConfigID int64 `json:"eab_config_id"`
		// ✅ 前端一直在传这两个 ✓，后端以前**根本没解析** ✗ → 选 ECC384/RSA 全无效 ✗
		KeyType string `json:"key_type"`
		KeyBits int    `json:"key_bits"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// ACME 账号邮箱：Google Public CA **必须有联系邮箱** ✗（真机报 `Accounts must have at least one contact.` ✓），
	// LE 也建议带上 ✓。选了账号就用选的，否则用面板里的**默认账号** ✓。
	email := ""
	if h.acmeEmails != nil {
		if info, rerr := h.acmeEmails.ResolveACMEAccount(c.Request.Context(), req.EmailID); rerr == nil {
			email = strings.TrimSpace(info.Email)
		}
	}
	if email == "" && req.EmailID > 0 {
		response.BadRequest(c, "ACME_ACCOUNT_NOT_FOUND", "the selected ACME account has no email")
		return
	}

	// 账号里存过 EAB（ZeroSSL / Google ✓）→ 自动补上 ✓，用户不用每次手填 ✗
	caProvider := strings.TrimSpace(req.CAProvider)
	eabKID := strings.TrimSpace(req.EABKeyID)
	eabMAC := strings.TrimSpace(req.EABMACKey)

	// ① 独立 EAB 配置（推荐用法 ✓：签发时只选一下 ✓）
	if req.EABConfigID > 0 {
		if h.eabConfigs == nil {
			response.InternalError(c, "NO_EAB_RESOLVER", "EAB config resolver is not configured on the panel")
			return
		}
		eb, eerr := h.eabConfigs.ResolveEABConfig(c.Request.Context(), req.EABConfigID)
		if eerr != nil || eb == nil {
			msg := "eab config not found"
			if eerr != nil {
				msg = eerr.Error()
			}
			response.BadRequest(c, "EAB_CONFIG_NOT_FOUND", msg)
			return
		}
		if strings.TrimSpace(eb.CAProvider) != "" {
			caProvider = strings.TrimSpace(eb.CAProvider)
		}
		if strings.TrimSpace(eb.KeyID) == "" || strings.TrimSpace(eb.MACKey) == "" {
			response.BadRequest(c, "EAB_CONFIG_INCOMPLETE",
				"EAB config '"+eb.Name+"' is missing eab_key_id or eab_mac_key")
			return
		}
		eabKID = strings.TrimSpace(eb.KeyID)
		eabMAC = strings.TrimSpace(eb.MACKey)
	}
	if caProvider == "" || eabMAC == "" {
		if h.acmeEmails == nil {
			// 用户明确选了账号却拿不到 → **必须报错** ✗（静默忽略会让"选了 Google 却按 LE 签"这种事发生 ✓）
			if req.EmailID > 0 {
				response.InternalError(c, "NO_ACME_RESOLVER", "ACME account resolver is not configured on the panel")
				return
			}
		} else {
			info, rerr := h.acmeEmails.ResolveACMEAccount(c.Request.Context(), req.EmailID)
			if rerr != nil || (info.CAProvider == "" && info.Email == "" && info.EABMACKey == "") {
				// 选了具体账号但解析不到 → 报错 ✓（否则会悄悄用默认 CA 去签 ✗）
				if req.EmailID > 0 {
					msg := "acme account not found"
					if rerr != nil {
						msg = rerr.Error()
					}
					response.BadRequest(c, "ACME_ACCOUNT_NOT_FOUND", msg)
					return
				}
			} else {
				if caProvider == "" {
					caProvider = strings.TrimSpace(info.CAProvider)
				}
				if eabKID == "" {
					eabKID = strings.TrimSpace(info.EABKeyID)
				}
				if eabMAC == "" {
					eabMAC = strings.TrimSpace(info.EABMACKey)
				}
			}
		}
	}

	// ③ **自动关联** ✓：CA 需要 EAB（Google / ZeroSSL ✓）而上面都没拿到 EAB 时 ✓，
	// 面板自动按 CA 挑一份已配置的 EAB ✓ —— 用户只需选 CA ✓，不用再管 EAB ✗。
	if eabMAC == "" && caNeedsEAB(caProvider) {
		if h.eabConfigs == nil {
			response.InternalError(c, "NO_EAB_RESOLVER", "EAB config resolver is not configured on the panel")
			return
		}
		cfg, aerr := h.eabConfigs.ResolveEABConfigForCA(c.Request.Context(), caProvider)
		if aerr != nil {
			response.InternalError(c, "EAB_LOOKUP_FAILED", aerr.Error())
			return
		}
		if cfg == nil && strings.EqualFold(strings.TrimSpace(caProvider), "google") {
			// Google：EAB 是 7 天 + 一次性 ✗ → 缺了就用**存过的 GCP 服务账号**现场再申请一把 ✓
			log.Printf("[cert] no usable Google EAB → trying GCP service account to create one")
			if made, fresh, gerr := h.eabConfigs.EnsureGoogleEAB(c.Request.Context()); gerr == nil && made != nil {
				if fresh {
					log.Printf("[cert] Google EAB auto-created via GCP: kid=%s", made.KeyID)
				}
				cfg = made
			} else if gerr != nil {
				log.Printf("[cert] auto-create Google EAB failed: %v", gerr)
			}
		}
		if cfg == nil && strings.EqualFold(strings.TrimSpace(caProvider), "zerossl") && strings.Contains(email, "@") {
			// ZeroSSL：没配过就**用邮箱自动领一对** ✓（同 acme.sh ✓）→ 首次申请即自动注册 ✓
			log.Printf("[cert] no ZeroSSL EAB configured → auto-fetching one for %s", email)
			if fetched, ferr := h.eabConfigs.FetchZeroSSLEAB(c.Request.Context(), email); ferr == nil && fetched != nil {
				cfg = fetched
			} else if ferr != nil {
				log.Printf("[cert] auto-fetch ZeroSSL EAB failed: %v", ferr)
			}
		}
		if cfg == nil {
			// 说清楚怎么办 ✓（否则只会看到 CA 那边一句 externalAccountRequired ✗ 很难排查 ✗）
			response.BadRequest(c, "EAB_CONFIG_REQUIRED",
				"CA '"+caProvider+"' requires EAB; please add one under EAB Configs first")
			return
		}
		if strings.TrimSpace(cfg.CAProvider) != "" {
			caProvider = strings.TrimSpace(cfg.CAProvider)
		}
		eabKID = strings.TrimSpace(cfg.KeyID)
		eabMAC = strings.TrimSpace(cfg.MACKey)
		log.Printf("[cert] EAB auto-associated by CA: ca=%q eab_config=%q kid=%s", caProvider, cfg.Name, eabKID)
	}

	input := certService.IssueACMEInput{
		Domains:       req.Domains,
		ChallengeType: req.ChallengeType,
		CAProvider:    cert.CAProvider(caProvider),
		Profile:       req.Profile,
		KeyType:       req.KeyType,
		KeyBits:       req.KeyBits,
		// ⚠ 必须把**选了哪条 DNS 通道**一起落库 ✗：续期要按"创建时的选择"走 ✓，
		// 而服务层只拿到解析后的凭据、认不出是哪条通道 ✗（真机 bug：续期时选择丢失 ✓）。
		DNSChannelID: req.DNSChannelID,
	}
	// 诊断：一眼看出本次用了哪个 CA / 有没有 EAB / 用哪个邮箱 ✓
	log.Printf("[cert] issue request: domains=%v challenge=%s ca=%q eab=%v eab_config_id=%d email_id=%d email=%q",
		req.Domains, req.ChallengeType, caProvider, eabKID != "" && eabMAC != "", req.EABConfigID, req.EmailID, email)
	input.Email = email

	if req.DNSProvider != nil {
		input.DNSProvider = &cert.DNSProviderConfig{
			Provider: cert.DNSProvider(req.DNSProvider.Provider),
			EnvVars:  req.DNSProvider.EnvVars,
		}
	} else if req.DNSChannelID > 0 {
		// 用面板里已保存的 DNS 通道 ✓（前端下拉选的就是它 ✓）
		if h.dnsChannels == nil {
			response.InternalError(c, "NO_DNS_CHANNEL_RESOLVER", "DNS channel resolver is not configured on the panel")
			return
		}
		dnsCfg, derr := h.dnsChannels.ResolveDNSChannel(c.Request.Context(), req.DNSChannelID)
		if derr != nil {
			response.BadRequest(c, "DNS_CHANNEL_NOT_FOUND", derr.Error())
			return
		}
		input.DNSProvider = dnsCfg
	}

	// 优先级：请求里显式给的 ✓/嵌套 ✓ > 账号里存的 ✓（上面的合并已经把账号值并进来了 ✓）
	if eabKID != "" || eabMAC != "" {
		input.EAB = &cert.EABConfig{KeyID: eabKID, MACKey: eabMAC}
	} else if req.EAB != nil {
		input.EAB = &cert.EABConfig{
			KeyID:  req.EAB.KeyID,
			MACKey: req.EAB.MACKey,
		}
	}

	cert, err := h.certSvc.IssueACME(c.Request.Context(), input)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_issue", strings.Join(req.Domains, ","), false, err.Error())
		// "同一张证书已经有一次签发在跑" ✓ 不是服务端故障 ✗ → 409 ✓：
		// 签发已被刻意脱离请求上下文 ✓（见 detachedCertOpContext ✓），上一次可能仍在后台跑 ✓，
		// 这里如实告诉调用方"稍后看结果" ✓，而不是让两个签发同时写同一份文件 ✗。
		if errors.Is(err, certService.ErrCertOpInProgress) {
			response.Conflict(c, "ISSUE_IN_PROGRESS", err.Error())
			return
		}
		response.InternalError(c, "ISSUE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_issue", fmt.Sprintf("issued cert %d: %s", cert.ID, cert.Domain), true, "")

	// Trigger distribution to nodes — new cert may be referenced by
	// configs that were previously using an older cert for the same domain.
	h.fireCertUpdated(cert.ID)
	log.Printf("[certificates] cert %d issued for %s, triggering distribution to nodes", cert.ID, cert.Domain)
	response.Created(c, cert)
}

// Renew handles POST /api/v1/certificates/:id/renew.
//
// 口径（用户口径 ✓）：**创建时选了什么，续期就用什么** ✓。
// 所以 body **可以完全不传** ✓ —— 挑战方式、DNS 通道、CA、profile 一律用签发时落库的值还原 ✓。
//
// ⚠ 以前这里不带任何参数就是致命伤 ✗：服务层只把 body 里的 dns_provider 往下传 ✗，
// 前端续期按钮又什么都不传 ✗ → 创建时选的 dns-01 + DNS 通道在续期时**凭空消失** ✗，
// 退化成 http-01 ✗（通配符直接失败 ✓，普通域名被偷偷换了验证方式 ✓）。
// 现在 body 里传的字段一律视为**显式覆盖** ✓，不传则沿用库里记录 ✓。
func (h *Handler) Renew(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}

	// 全部字段可选 ✓（不带 body 也能续 ✓）
	var req struct {
		ChallengeType string `json:"challenge_type"`
		DNSChannelID  int64  `json:"dns_channel_id"`
		DNSProvider   *struct {
			Provider string            `json:"provider"`
			EnvVars  map[string]string `json:"env_vars"`
		} `json:"dns_provider,omitempty"`
	}
	c.ShouldBindJSON(&req)

	in := certService.RenewACMEInput{
		ChallengeType: req.ChallengeType,
		DNSChannelID:  req.DNSChannelID,
	}
	if req.DNSProvider != nil {
		in.DNSProvider = &cert.DNSProviderConfig{
			Provider: cert.DNSProvider(req.DNSProvider.Provider),
			EnvVars:  req.DNSProvider.EnvVars,
		}
	}

	// 邮箱 / EAB：与签发**同源** ✓（只读，不现场申请 ✗，理由见 resolveRenewalAuth 注释 ✓）
	if rec, gerr := h.certSvc.GetByID(c.Request.Context(), id); gerr == nil && rec != nil {
		in.Email, in.EAB = h.resolveRenewalAuth(c.Request.Context(), rec.Issuer)
	}

	// 诊断：一眼看出本次按什么方式续 ✓（不含任何凭据 ✓）
	log.Printf("[cert] renew request: id=%d challenge=%q dns_channel_id=%d dns_provider_given=%v eab=%v email=%q",
		id, in.ChallengeType, in.DNSChannelID, in.DNSProvider != nil, in.EAB != nil, in.Email)

	cert, err := h.certSvc.RenewACME(c.Request.Context(), id, in)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_renew", fmt.Sprintf("cert %d", id), false, err.Error())
		// 同上 ✓：续期同样在后台保证跑完 ✓ → 并发的第二次直接 409 ✓（见 handler 里 Issue 的注释 ✓）。
		if errors.Is(err, certService.ErrCertOpInProgress) {
			response.Conflict(c, "RENEW_IN_PROGRESS", err.Error())
			return
		}
		response.InternalError(c, "RENEW_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_renew", fmt.Sprintf("renewed cert %d: %s", id, cert.Domain), true, "")

	h.fireCertUpdated(id)
	log.Printf("[certificates] cert %d renewed (challenge=%s dns_channel=%d), triggering distribution to nodes",
		id, cert.ChallengeType, cert.DNSChannelID)
	response.OK(c, cert)
}

// resolveRenewalAuth 返回续期应使用的 ACME 邮箱与 EAB ✓ —— 口径与签发一致 ✓。
//
// ⚠ 这里**故意只读、不现场申请** EAB ✗：
// Google 的 EAB 是"7 天 + 一次性" ✓，若每次续期都去 mint 一把 → 白白消耗配额 ✗，
// 还有可能把正在用的那把挤作废 ✗。首次注册确实需要 EAB ✓，
// 但那种场景（面板本机没有该 CA 的账号 ✓）应该走"重新签发"或先配好账号 ✗，不该由续期默默代劳 ✗。
//
// 因此取用顺序（都不产生副作用 ✓）：
//  1. 默认账号里 CA 对得上的 EAB ✓；
//  2. 按 CA 关联的 EAB 配置 ✓（`acme_account_configs` / `eab_configs` ✓）。
//
// 取不到就返回 nil ✓：已有账号的续期根本用不到 EAB ✓（注册会被跳过 ✓），
// 真需要首次注册时 CA 会明确报错 ✓（比静默换个验证方式好得多 ✓）。
func (h *Handler) resolveRenewalAuth(ctx context.Context, caProvider string) (string, *cert.EABConfig) {
	var email string
	var eab *cert.EABConfig

	if h.acmeEmails != nil {
		if info, err := h.acmeEmails.ResolveACMEAccount(ctx, 0); err == nil {
			email = strings.TrimSpace(info.Email)
			if caNeedsEAB(caProvider) &&
				info.EABKeyID != "" &&
				strings.EqualFold(strings.TrimSpace(info.CAProvider), strings.TrimSpace(caProvider)) {
				eab = &cert.EABConfig{KeyID: info.EABKeyID, MACKey: info.EABMACKey}
			}
		}
	}
	if eab != nil || !caNeedsEAB(caProvider) || h.eabConfigs == nil {
		return email, eab
	}

	if cfg, err := h.eabConfigs.ResolveEABConfigForCA(ctx, caProvider); err == nil && cfg != nil && cfg.KeyID != "" {
		eab = &cert.EABConfig{KeyID: cfg.KeyID, MACKey: cfg.MACKey}
	}
	return email, eab
}

// FindExpiringSoon returns certificates expiring within 30 days.
func (h *Handler) FindExpiringSoon(c *gin.Context) {
	certs, err := h.certSvc.FindExpiringSoon(c.Request.Context(), 30*24*time.Hour)
	if err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}

	response.OK(c, certs)
}

// DetectIP handles GET /api/v1/certificates/detect-ip.
func (h *Handler) DetectIP(c *gin.Context) {
	ipv4, ipv6, err := h.certSvc.DetectPublicIP()
	if err != nil {
		response.InternalError(c, "DETECT_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"ipv4": ipv4,
		"ipv6": ipv6,
	})
}

// CheckPort handles GET /api/v1/certificates/check-port.
func (h *Handler) CheckPort(c *gin.Context) {
	port := c.DefaultQuery("port", "80")
	inUse, processInfo := h.certSvc.CheckPortInUse(port)
	response.OK(c, gin.H{
		"port":    port,
		"in_use":  inUse,
		"process": processInfo,
	})
}

// Upload handles POST /api/v1/certificates/upload.
func (h *Handler) Upload(c *gin.Context) {
	var input certService.UploadCertificateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	cert, err := h.certSvc.Upload(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "UPLOAD_FAILED", err.Error())
		return
	}

	// Trigger distribution — uploaded cert may replace an existing one
	// referenced by node configs with the same domain.
	h.fireCertUpdated(cert.ID)
	log.Printf("[certificates] cert %d uploaded for %s, triggering distribution to nodes", cert.ID, cert.Domain)
	response.Created(c, cert)
}
