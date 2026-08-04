package certificates

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/service/audit"
	certService "corefusion/internal/service/certificate"
)

// CertUpdateCallback is called after a certificate is updated or renewed.
// It allows external code (e.g. router wiring) to trigger distribution to nodes.
type CertUpdateCallback func(ctx context.Context, certID int64)

// Handler handles certificate HTTP requests.
type Handler struct {
	certSvc       *certService.Service
	onCertUpdated CertUpdateCallback
}

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

// Issue handles POST /api/v1/certificates/issue.
func (h *Handler) Issue(c *gin.Context) {
	var req struct {
		Domains       []string `json:"domains" binding:"required"`
		ChallengeType string   `json:"challenge_type"`
		CAProvider    string   `json:"ca_provider"`
		DNSProvider   *struct {
			Provider string            `json:"provider"`
			EnvVars  map[string]string `json:"env_vars"`
		} `json:"dns_provider,omitempty"`
		EAB *struct {
			KeyID  string `json:"key_id"`
			MACKey string `json:"mac_key"`
		} `json:"eab,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	input := certService.IssueACMEInput{
		Domains:       req.Domains,
		ChallengeType: req.ChallengeType,
		CAProvider:    cert.CAProvider(req.CAProvider),
	}

	if req.DNSProvider != nil {
		input.DNSProvider = &cert.DNSProviderConfig{
			Provider: cert.DNSProvider(req.DNSProvider.Provider),
			EnvVars:  req.DNSProvider.EnvVars,
		}
	}

	if req.EAB != nil {
		input.EAB = &cert.EABConfig{
			KeyID:  req.EAB.KeyID,
			MACKey: req.EAB.MACKey,
		}
	}

	cert, err := h.certSvc.IssueACME(c.Request.Context(), input)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_issue", strings.Join(req.Domains, ","), false, err.Error())
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
func (h *Handler) Renew(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}

	// DNS provider config is optional, passed when using DNS-01 challenge
	var req struct {
		DNSProvider *struct {
			Provider string            `json:"provider"`
			EnvVars  map[string]string `json:"env_vars"`
		} `json:"dns_provider,omitempty"`
	}
	c.ShouldBindJSON(&req)

	var dnsConfig *cert.DNSProviderConfig
	if req.DNSProvider != nil {
		dnsConfig = &cert.DNSProviderConfig{
			Provider: cert.DNSProvider(req.DNSProvider.Provider),
			EnvVars:  req.DNSProvider.EnvVars,
		}
	}

	cert, err := h.certSvc.RenewACME(c.Request.Context(), id, dnsConfig)
	if err != nil {
		audit.RecordNodeFromCtx(c, 0, "cert_renew", fmt.Sprintf("cert %d", id), false, err.Error())
		response.InternalError(c, "RENEW_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, 0, "cert_renew", fmt.Sprintf("renewed cert %d: %s", id, cert.Domain), true, "")

	h.fireCertUpdated(id)
	log.Printf("[certificates] cert %d renewed, triggering distribution to nodes", id)
	response.OK(c, cert)
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
		"port":       port,
		"in_use":     inUse,
		"process":    processInfo,
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
