package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// BootstrapTokenManager manages Bootstrap Token generation and validation.
// Tokens are one-time use, expire after 24 hours, and are stored as SHA-256 hashes.
type BootstrapTokenManager struct {
	serverRepo *repository.NodeServerRepository
	caService  *CertificateAuthority
}

// NewBootstrapTokenManager creates a new BootstrapTokenManager.
func NewBootstrapTokenManager(serverRepo *repository.NodeServerRepository, ca *CertificateAuthority) *BootstrapTokenManager {
	return &BootstrapTokenManager{
		serverRepo: serverRepo,
		caService:  ca,
	}
}

// GenerateBootstrapToken generates a one-time registration token for a server.
// Called only when an admin adds a new server. Returns the plaintext token once.
func (m *BootstrapTokenManager) GenerateBootstrapToken(ctx context.Context, serverID int64) (string, error) {
	token, err := generateSecureToken(32) // 256-bit
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Store hash, not plaintext
	server.BootstrapToken = hashToken(token)
	server.BootstrapTokenExpiresAt = &expiresAt
	server.BootstrapUsedAt = nil

	if err := m.serverRepo.Update(ctx, server); err != nil {
		return "", fmt.Errorf("update server: %w", err)
	}

	log.Printf("[Bootstrap] token generated for server %d, expires at %v", serverID, expiresAt)
	return token, nil // Return plaintext to admin, only once
}

// ValidateBootstrapToken validates an Agent's Bootstrap Token.
// On success, issues an mTLS certificate and invalidates the token.
func (m *BootstrapTokenManager) ValidateBootstrapToken(ctx context.Context, serverID int64, token string) (*AgentCertResult, error) {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Check if token already used
	if server.BootstrapUsedAt != nil {
		return nil, fmt.Errorf("bootstrap token already used at %v", server.BootstrapUsedAt)
	}

	// Check if token expired
	if server.BootstrapTokenExpiresAt == nil || time.Now().After(*server.BootstrapTokenExpiresAt) {
		return nil, fmt.Errorf("bootstrap token expired or not set")
	}

	// Verify token hash
	if server.BootstrapToken == "" || !verifyToken(token, server.BootstrapToken) {
		return nil, fmt.Errorf("invalid bootstrap token")
	}

	// Issue Agent mTLS certificate
	certResult, err := m.caService.IssueAgentCertificate(ctx, serverID, server.Hostname)
	if err != nil {
		return nil, fmt.Errorf("issue agent certificate: %w", err)
	}

	// Invalidate token
	now := time.Now()
	server.BootstrapUsedAt = &now
	server.BootstrapToken = "" // Clear hash
	server.AgentCertSerial = certResult.Serial
	server.AgentCertExpiresAt = &certResult.ExpiresAt
	server.AgentCertLastRenewedAt = &now
	server.AgentStatus = entity.AgentStatusOnline

	if err := m.serverRepo.Update(ctx, server); err != nil {
		return nil, fmt.Errorf("update server after bootstrap: %w", err)
	}

	log.Printf("[Bootstrap] token validated for server %d, agent cert issued (serial=%s, expires=%v)",
		serverID, certResult.Serial, certResult.ExpiresAt)

	return certResult, nil
}

// VerifyToken checks if a bootstrap token is valid without consuming it.
// Used by the install script endpoint to verify the request is legitimate.
func (m *BootstrapTokenManager) VerifyToken(ctx context.Context, serverID int64, token string) bool {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return false
	}
	if server.BootstrapUsedAt != nil {
		return false
	}
	if server.BootstrapTokenExpiresAt == nil || time.Now().After(*server.BootstrapTokenExpiresAt) {
		return false
	}
	return server.BootstrapToken != "" && verifyToken(token, server.BootstrapToken)
}

// IsBootstrapRequired checks if a server needs bootstrap (no cert or token not yet used).
func (m *BootstrapTokenManager) IsBootstrapRequired(ctx context.Context, serverID int64) (bool, error) {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return false, err
	}

	// If cert already issued, no bootstrap needed
	if server.AgentCertSerial != "" && server.AgentCertExpiresAt != nil {
		return false, nil
	}

	// If token not yet used and not expired, bootstrap is required
	if server.BootstrapUsedAt == nil &&
		server.BootstrapTokenExpiresAt != nil &&
		time.Now().Before(*server.BootstrapTokenExpiresAt) {
		return true, nil
	}

	// Token expired or never generated
	return false, nil
}

// generateSecureToken generates a cryptographically secure random hex token.
func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the SHA-256 hex hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// verifyToken checks if a plaintext token matches the stored hash.
func verifyToken(token, hashed string) bool {
	return hashToken(token) == hashed
}
