package passkey

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/auth"
	"corefusion/internal/service/passkey"
	"corefusion/pkg/netutil"
)

// Handler 处理 Passkey 的 HTTP 请求。
type Handler struct {
	svc     *passkey.Service
	authSvc *auth.Service
}

// NewHandler 创建 Passkey 处理器。
func NewHandler(svc *passkey.Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

// recordLoginFailure 把登录失败计入登录限流（与密码登录一致的防爆破手段）。
func recordLoginFailure(c *gin.Context) {
	middleware.DefaultLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
}

// deriveOrigin 从请求推导 (scheme, host, origin)。
// 与 oauth/handler.go 的写法保持一致：优先信任 X-Forwarded-Proto / X-Forwarded-Host。
func deriveOrigin(c *gin.Context) (scheme, host, origin string) {
	scheme = "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host = c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme, host, scheme + "://" + host
}

// Status handles GET /api/v1/auth/passkey/status（公开）。
// 返回当前访问地址是否支持 Passkey（命中白名单 + 安全上下文），供前端决定是否显示按钮。
func (h *Handler) Status(c *gin.Context) {
	scheme, host, origin := deriveOrigin(c)
	rpid := h.svc.ResolveRPID(c.Request.Context(), host)
	response.OK(c, gin.H{
		"available": rpid != "" && passkey.IsSecureContext(scheme, host),
		"rpid":      rpid,
		"origin":    origin,
		"secure":    passkey.IsSecureContext(scheme, host),
	})
}

// RPID status checking helper（登录前后端都会查 status，未认证接口保持唯一次数，不必限流）.

type loginBeginRequest struct {
	// 目前走 discoverable（无用户名）流程；字段保留以便将来按用户名缩小 allowCredentials。
	Username string `json:"username"`
}

// LoginBegin handles POST /api/v1/auth/passkey/login/begin（公开）。
func (h *Handler) LoginBegin(c *gin.Context) {
	var req loginBeginRequest
	_ = c.ShouldBindJSON(&req) // body 可为空

	_, host, origin := deriveOrigin(c)
	assertion, token, err := h.svc.BeginLogin(c.Request.Context(), host, origin)
	if err != nil {
		response.BadRequest(c, "PASSKEY_UNAVAILABLE", err.Error())
		return
	}
	response.OK(c, gin.H{
		"session_token": token,
		// go-webauthn 的 CredentialAssertion 序列化后是 {"publicKey": {...}}；
		// 这里只返回 .Response（标准 PublicKeyCredentialRequestOptionsJSON），
		// 与前端契约保持一致：options 即标准的 RequestOptionsJSON。
		"options": assertion.Response,
	})
}

type loginFinishRequest struct {
	SessionToken string          `json:"session_token" binding:"required"`
	Response     json.RawMessage `json:"response" binding:"required"`
}

// LoginFinish handles POST /api/v1/auth/passkey/login/finish（公开）。
// 验证通过后签发与密码登录完全一致的 token（同一套 token_version 吊销机制）。
func (h *Handler) LoginFinish(c *gin.Context) {
	var req loginFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	user, err := h.svc.FinishLogin(c.Request.Context(), req.SessionToken, req.Response)
	if err != nil {
		recordLoginFailure(c)
		audit.Record(audit.Event{
			Category: "auth", EventType: "passkey_login", Result: "failure",
			RemoteIP: c.ClientIP(), Reason: err.Error(),
		})
		response.Unauthorized(c, "PASSKEY_LOGIN_FAILED", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authSvc.IssueTokens(c.Request.Context(), user)
	if err != nil {
		response.InternalError(c, "TOKEN_ISSUE_FAILED", err.Error())
		return
	}
	middleware.DefaultLoginLimiter().RecordSuccess(netutil.GetRealIP(c.Request))
	audit.Record(audit.Event{
		Category: "auth", EventType: "passkey_login", Result: "success",
		UserID: user.ID, RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"username": user.Username},
	})

	response.OK(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

type registerBeginRequest struct {
	Name string `json:"name"` // 凭证名称，可选
}

// RegisterBegin handles POST /api/v1/auth/passkey/register/begin（需已登录）。
func (h *Handler) RegisterBegin(c *gin.Context) {
	var req registerBeginRequest
	_ = c.ShouldBindJSON(&req)

	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	_, host, origin := deriveOrigin(c)
	creation, token, err := h.svc.BeginRegistration(c.Request.Context(), user, host, origin)
	if err != nil {
		response.BadRequest(c, "PASSKEY_UNAVAILABLE", err.Error())
		return
	}
	response.OK(c, gin.H{
		"session_token": token,
		// 与 LoginBegin 同理：返回 .Response（标准 PublicKeyCredentialCreationOptionsJSON）。
		"options": creation.Response,
	})
}

type registerFinishRequest struct {
	SessionToken string          `json:"session_token" binding:"required"`
	Response     json.RawMessage `json:"response" binding:"required"`
	Name         string          `json:"name"`
}

// RegisterFinish handles POST /api/v1/auth/passkey/register/finish（需已登录）。
func (h *Handler) RegisterFinish(c *gin.Context) {
	var req registerFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	cred, err := h.svc.FinishRegistration(c.Request.Context(), user, req.SessionToken, req.Response, req.Name)
	if err != nil {
		response.BadRequest(c, "PASSKEY_REGISTER_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "passkey_register", Result: "success",
		UserID: user.ID, RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"credential_id": cred.CredentialID[:16]},
	})
	creds, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		creds = nil
	}
	response.OK(c, gin.H{"credential": cred, "credentials": creds})
}

// ListCredentials handles GET /api/v1/auth/passkey/credentials（需已登录）。
func (h *Handler) ListCredentials(c *gin.Context) {
	userID := c.GetInt64("user_id")
	creds, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, creds)
}

// DeleteCredential handles DELETE /api/v1/auth/passkey/credentials/:id（需已登录）。
func (h *Handler) DeleteCredential(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.DeleteCredential(c.Request.Context(), userID, uri.ID); err != nil {
		response.BadRequest(c, "PASSKEY_DELETE_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "passkey_remove", Result: "success",
		UserID: userID, RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"credential_id": uri.ID},
	})
	response.OK(c, gin.H{"message": "credential removed"})
}

type rpidsRequest struct {
	Action string `json:"action" binding:"required,oneof=add remove"` // add | remove
	RPID   string `json:"rpid" binding:"required"`
}

// UpdateRPIDs handles POST /api/v1/auth/passkey/rpids（管理员）。
// 把当前访问域名加入/移出白名单 —— 零配置起步：管理员在设置页确认一次。
func (h *Handler) UpdateRPIDs(c *gin.Context) {
	var req rpidsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	ctx := c.Request.Context()
	var err error
	switch req.Action {
	case "add":
		err = h.svc.AddRPID(ctx, req.RPID)
	case "remove":
		err = h.svc.RemoveRPID(ctx, req.RPID)
	}
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"rpid": req.RPID, "rpids": h.svc.RPIDs(ctx)})
}

// ListRPIDs handles GET /api/v1/auth/passkey/rpids（管理员）。
func (h *Handler) ListRPIDs(c *gin.Context) {
	response.OK(c, h.svc.RPIDs(c.Request.Context()))
}