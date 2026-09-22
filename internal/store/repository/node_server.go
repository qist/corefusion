package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/constants"
	"corefusion/internal/model/entity"
)

// NodeServerRepository handles node_servers database operations.
type NodeServerRepository struct {
	db *gorm.DB
}

// NewNodeServerRepository creates a new NodeServerRepository.
func NewNodeServerRepository(db *gorm.DB) *NodeServerRepository {
	return &NodeServerRepository{db: db}
}

// Create creates a new node server record.
func (r *NodeServerRepository) Create(ctx context.Context, server *entity.NodeServer) error {
	return r.db.WithContext(ctx).Create(server).Error
}

// GetByID retrieves a node server by ID.
func (r *NodeServerRepository) GetByID(ctx context.Context, id int64) (*entity.NodeServer, error) {
	var server entity.NodeServer
	err := r.db.WithContext(ctx).First(&server, id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// GetByHostname retrieves a node server by hostname.
func (r *NodeServerRepository) GetByHostname(ctx context.Context, hostname string) (*entity.NodeServer, error) {
	var server entity.NodeServer
	err := r.db.WithContext(ctx).Where("hostname = ?", hostname).First(&server).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// ListAll returns all node servers ordered by sort_order.
func (r *NodeServerRepository) ListAll(ctx context.Context) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Order("sort_order ASC, id ASC").
		Find(&servers).Error
	return servers, err
}

// ListEnabled returns all enabled node servers.
func (r *NodeServerRepository) ListEnabled(ctx context.Context) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&servers).Error
	return servers, err
}

// ListByRegion returns node servers in a specific region.
func (r *NodeServerRepository) ListByRegion(ctx context.Context, region string) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("region = ? AND enabled = ?", region, true).
		Order("sort_order ASC, id ASC").
		Find(&servers).Error
	return servers, err
}

// ListByType returns node servers of a specific type.
func (r *NodeServerRepository) ListByType(ctx context.Context, nodeType string) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("node_type = ?", nodeType).
		Order("sort_order ASC, id ASC").
		Find(&servers).Error
	return servers, err
}

// ListByPartner returns node servers paired with the given server ID.
func (r *NodeServerRepository) ListByPartner(ctx context.Context, partnerID int64) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("partner_server_id = ?", partnerID).
		Find(&servers).Error
	return servers, err
}

// ListBillingEnabled returns all node servers that participate in billing.
func (r *NodeServerRepository) ListBillingEnabled(ctx context.Context) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND billing_enabled = ?", true, true).
		Order("sort_order ASC, id ASC").
		Find(&servers).Error
	return servers, err
}

// ListCertExpiringSoon returns servers whose Agent cert expires within the given threshold.
func (r *NodeServerRepository) ListCertExpiringSoon(ctx context.Context, before time.Time) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("is_local = ? AND agent_cert_expires_at IS NOT NULL AND agent_cert_expires_at < ?",
			false, before).
		Find(&servers).Error
	return servers, err
}

// ListLocal returns the local node server (is_local = true).
func (r *NodeServerRepository) ListLocal(ctx context.Context) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).
		Where("is_local = ?", true).
		Find(&servers).Error
	return servers, err
}

// Update saves all fields of a node server.
func (r *NodeServerRepository) Update(ctx context.Context, server *entity.NodeServer) error {
	return r.db.WithContext(ctx).Save(server).Error
}

// UpdateAgentStatus updates only the agent status and heartbeat.
func (r *NodeServerRepository) UpdateAgentStatus(ctx context.Context, id int64, status string, version string) error {
	updates := map[string]interface{}{
		"agent_status":      status,
		"agent_version":     version,
		"last_heartbeat_at": time.Now(),
		"updated_at":        time.Now(),
	}
	return r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateHeartbeat updates only the last heartbeat timestamp.
func (r *NodeServerRepository) UpdateHeartbeat(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_heartbeat_at": time.Now(),
			"updated_at":        time.Now(),
		}).Error
}

// UpdateAgentCert updates the Agent certificate info for a server.
func (r *NodeServerRepository) UpdateAgentCert(ctx context.Context, id int64, serial string, expiresAt time.Time) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"agent_cert_serial":          serial,
			"agent_cert_expires_at":      expiresAt,
			"agent_cert_last_renewed_at": now,
			"updated_at":                 now,
		}).Error
}

// UpdateCertificateID updates the associated certificate ID for a server.
func (r *NodeServerRepository) UpdateCertificateID(ctx context.Context, id int64, certID int64) error {
	return r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"certificate_id": certID,
			"updated_at":     time.Now(),
		}).Error
}

// Delete removes a node server by ID.
func (r *NodeServerRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.NodeServer{}, id).Error
}

// Count returns the total number of node servers.
func (r *NodeServerRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.NodeServer{}).Count(&count).Error
	return count, err
}

// ListAllServerIDs returns all server IDs.
func (r *NodeServerRepository) ListAllServerIDs(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Pluck("id", &ids).Error
	return ids, err
}

// EnsureLocalServer ensures a local node server record exists.
// Called during panel bootstrap to represent the panel's own machine.
func (r *NodeServerRepository) EnsureLocalServer(ctx context.Context, hostname string) error {
	var count int64
	r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Where("is_local = ?", true).Count(&count)
	if count > 0 {
		return nil
	}

	now := time.Now()
	local := &entity.NodeServer{
		Name:           "Local (Panel)",
		Hostname:       hostname,
		NodeType:       entity.NodeTypeStandalone,
		AgentStatus:    entity.AgentStatusOnline,
		AgentPort:      constants.DefaultAgentGRPCPort,
		IsLocal:        true,
		BillingEnabled: true,
		SupportedCores: `["xray","singbox","hysteria2","tuic"]`,
		Enabled:        true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return r.db.WithContext(ctx).Create(local).Error
}

// ListByCertificateID 返回引用了某张证书的全部节点（"这张证书要发给谁" ✓）。
// 公网证书的自动续期要靠它找到"该让哪个节点在本机续" ✓（node_servers.certificate_id 是现成绑定 ✓）。
func (r *NodeServerRepository) ListByCertificateID(ctx context.Context, certID int64) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	err := r.db.WithContext(ctx).Where("certificate_id = ?", certID).Find(&servers).Error
	return servers, err
}
