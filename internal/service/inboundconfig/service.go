package inboundconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	"corefusion/internal/store/repository"
)

// ErrServerNamesConflict is returned when a reality inbound tries to reuse a
// serverName (SNI) that is already used by another inbound on the same machine
// (node-server). A SNI must be unique per machine, otherwise the SNI routing
// cannot decide which inbound to forward to.
var ErrServerNamesConflict = errors.New("server names conflict")

// Service handles inbound config business logic.
type Service struct {
	repo *repository.InboundConfigRepository
	db   *gorm.DB
}

// NewService creates a new inbound config service.
func NewService(repo *repository.InboundConfigRepository, db *gorm.DB) *Service {
	return &Service{repo: repo, db: db}
}

// List returns all inbound configs for a core type on a specific server.
func (s *Service) List(ctx context.Context, serverID int64, coreType string) ([]entity.InboundConfig, error) {
	return s.repo.List(ctx, serverID, coreType)
}

// ListAll returns all inbound configs across all servers (for traffic sync).
func (s *Service) ListAll(ctx context.Context) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	err := s.db.WithContext(ctx).Order("id").Find(&configs).Error
	return configs, err
}

// GetByID returns an inbound config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new inbound config.
func (s *Service) Create(ctx context.Context, serverID int64, coreType, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	if serverID == 0 {
		serverID = 1 // default to local node
	}
	if snis := extractServerNames(configJSON); len(snis) > 0 {
		if err := s.checkServerNamesConflict(ctx, serverID, 0, snis); err != nil {
			return nil, err
		}
	}
	cfg := &entity.InboundConfig{
		ServerID:   serverID,
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		Listen:     listen,
		Port:       port,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an existing inbound config.
func (s *Service) Update(ctx context.Context, id int64, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if snis := extractServerNames(configJSON); len(snis) > 0 {
		if err := s.checkServerNamesConflict(ctx, cfg.ServerID, id, snis); err != nil {
			return nil, err
		}
	}
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.Listen = listen
	cfg.Port = port
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Delete deletes an inbound config.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// extractServerNames parses the reality serverNames (SNI) from an inbound
// config JSON. It reads both the array form ("serverNames") and the
// newline/comma separated string form ("serverNamesStr"), returning a
// lower-cased, de-duplicated slice. Returns nil when the inbound is not a
// reality inbound or has no serverNames.
func extractServerNames(cfgJSON string) []string {
	var inbound struct {
		StreamSettings struct {
			Security string `json:"security"`
			Reality  struct {
				ServerNames    []string `json:"serverNames"`
				ServerNamesStr string   `json:"serverNamesStr"`
			} `json:"realitySettings"`
		} `json:"streamSettings"`
	}
	if err := json.Unmarshal([]byte(cfgJSON), &inbound); err != nil {
		return nil
	}
	if strings.TrimSpace(inbound.StreamSettings.Security) != "reality" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range inbound.StreamSettings.Reality.ServerNames {
		add(s)
	}
	for _, s := range strings.FieldsFunc(inbound.StreamSettings.Reality.ServerNamesStr, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	}) {
		add(s)
	}
	return out
}

// checkServerNamesConflict verifies that none of the given serverNames are
// already used by another inbound on the same machine (node-server).
// excludeID is the inbound being updated (0 when creating).
func (s *Service) checkServerNamesConflict(ctx context.Context, serverID, excludeID int64, newSNIs []string) error {
	existing, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return err
	}
	newSet := make(map[string]struct{}, len(newSNIs))
	for _, sn := range newSNIs {
		newSet[sn] = struct{}{}
	}
	for _, cfg := range existing {
		if cfg.ID == excludeID {
			continue
		}
		for _, sn := range extractServerNames(cfg.ConfigJSON) {
			if _, ok := newSet[sn]; ok {
				return fmt.Errorf("%w: server name %q already used by inbound %q on this machine", ErrServerNamesConflict, sn, cfg.Remark)
			}
		}
	}
	return nil
}

// UpdateTraffic updates traffic stats for a user by email.
func (s *Service) UpdateTraffic(ctx context.Context, tag string, email string, upDelta int64, downDelta int64) error {
	// Find all inbound configs across all servers
	configs, err := s.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}

		// Check if this inbound has the user
		settings, ok := inbound["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		updated := false
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := settings[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				// Match by email or user field
				itemEmail, _ := m["email"].(string)
				if itemEmail == "" {
					itemEmail, _ = m["user"].(string)
				}
				if itemEmail == email {
					// Update traffic
					up, _ := m["_usedUp"].(float64)
					down, _ := m["_usedDown"].(float64)
					m["_usedUp"] = int64(up) + upDelta
					m["_usedDown"] = int64(down) + downDelta
					updated = true
				}
			}
		}

		if updated {
			data, err := json.Marshal(inbound)
			if err != nil {
				log.Printf("trafficsync: marshal failed for %s: %v", cfg.Tag, err)
				continue
			}
			cfg.ConfigJSON = string(data)
			cfg.UpdatedAt = time.Now()
			log.Printf("trafficsync: saving config %s (id=%d)", cfg.Tag, cfg.ID)
			if err := s.repo.Update(ctx, &cfg); err != nil {
				log.Printf("trafficsync: failed to update config %s: %v", cfg.Tag, err)
			} else {
				log.Printf("trafficsync: saved config %s successfully", cfg.Tag)
			}
			// Also update subscription traffic caches
			s.updateSubscriptionTraffic(ctx, email, upDelta, downDelta)
		}
	}
	return nil
}

// updateSubscriptionTraffic finds the subscription associated with the given
// user email and increments its traffic cache columns.
// The email from xray stats is in the format "user@email.com_inboundTag",
// so we extract the real email by finding the last "_" and taking everything before it
// (only if the part after "_" looks like a tag, i.e. the email contains "@").
func (s *Service) updateSubscriptionTraffic(ctx context.Context, statsEmail string, upDelta, downDelta int64) {
	if s.db == nil || (upDelta == 0 && downDelta == 0) {
		return
	}

	// Extract real email from "user@email.com_tag" format
	realEmail := statsEmail
	if idx := strings.LastIndex(statsEmail, "_"); idx > 0 {
		candidate := statsEmail[:idx]
		if strings.Contains(candidate, "@") {
			realEmail = candidate
		}
	}

	total := upDelta + downDelta

	// Find client by email, then use their subscription_id
	var client entity.Client
	if err := s.db.WithContext(ctx).Where("email = ?", realEmail).First(&client).Error; err != nil {
		return
	}
	if client.SubscriptionID == nil || *client.SubscriptionID == 0 {
		return
	}

	err := s.db.WithContext(ctx).Model(&entity.Subscription{}).
		Where("id = ?", *client.SubscriptionID).
		Updates(map[string]interface{}{
			"used_bytes_cache":     gorm.Expr("used_bytes_cache + ?", total),
			"upload_bytes_cache":   gorm.Expr("upload_bytes_cache + ?", upDelta),
			"download_bytes_cache": gorm.Expr("download_bytes_cache + ?", downDelta),
		}).Error
	if err != nil {
		log.Printf("trafficsync: failed to update subscription %d traffic: %v", *client.SubscriptionID, err)
	}

	// Update client's last_node_usage_at to reflect real-time activity
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Update("last_node_usage_at", now).Error; err != nil {
		log.Printf("trafficsync: failed to update client %d last_node_usage_at: %v", client.ID, err)
	}
}

// ResetAllTraffic resets _usedUp and _usedDown to 0 for all users in all
// inbound configs on the given server. Also resets subscription traffic
// cache columns for affected clients.
func (s *Service) ResetAllTraffic(ctx context.Context, serverID int64) (int, error) {
	configs, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return 0, err
	}

	resetCount := 0
	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}

		settings, ok := inbound["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		configReset := false
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := settings[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if _, hasUp := m["_usedUp"]; hasUp {
					m["_usedUp"] = int64(0)
					configReset = true
					resetCount++
				}
				if _, hasDown := m["_usedDown"]; hasDown {
					m["_usedDown"] = int64(0)
					configReset = true
				}
			}
		}

		if configReset {
			data, err := json.Marshal(inbound)
			if err != nil {
				log.Printf("resetTraffic: marshal failed for %s: %v", cfg.Tag, err)
				continue
			}
			cfg.ConfigJSON = string(data)
			cfg.UpdatedAt = time.Now()
			if err := s.repo.Update(ctx, &cfg); err != nil {
				log.Printf("resetTraffic: failed to update config %s: %v", cfg.Tag, err)
			}
		}
	}

	// Reset subscription traffic caches for all clients on this server
	s.resetSubscriptionTrafficForServer(ctx, serverID)

	return resetCount, nil
}

// resetSubscriptionTrafficForServer resets the traffic cache columns
// (used_bytes_cache, upload_bytes_cache, download_bytes_cache) for all
// subscriptions that have clients associated with the given server's inbounds.
func (s *Service) resetSubscriptionTrafficForServer(ctx context.Context, serverID int64) {
	if s.db == nil {
		return
	}

	// Find all client emails from this server's inbound configs
	configs, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return
	}

	emailSet := make(map[string]bool)
	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}
		settings, ok := inbound["settings"].(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := settings[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				email, _ := m["email"].(string)
				if email == "" {
					email, _ = m["user"].(string)
				}
				if email != "" {
					// Extract real email from "user@email.com_tag" format
					realEmail := email
					if idx := strings.LastIndex(email, "_"); idx > 0 {
						candidate := email[:idx]
						if strings.Contains(candidate, "@") {
							realEmail = candidate
						}
					}
					emailSet[realEmail] = true
				}
			}
		}
	}

	if len(emailSet) == 0 {
		return
	}

	// Collect unique subscription IDs for these clients
	emails := make([]string, 0, len(emailSet))
	for email := range emailSet {
		emails = append(emails, email)
	}

	var subIDs []int64
	s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("email IN ?", emails).
		Where("subscription_id IS NOT NULL AND subscription_id > 0").
		Distinct("subscription_id").
		Pluck("subscription_id", &subIDs)

	if len(subIDs) == 0 {
		return
	}

	// Reset traffic cache for these subscriptions
	s.db.WithContext(ctx).Model(&entity.Subscription{}).
		Where("id IN ?", subIDs).
		Updates(map[string]interface{}{
			"used_bytes_cache":     0,
			"upload_bytes_cache":   0,
			"download_bytes_cache": 0,
		})

	log.Printf("resetTraffic: reset traffic cache for %d subscriptions on server %d", len(subIDs), serverID)
}

// ResetUserTrafficByEmail resets _usedUp and _usedDown to 0 for a specific user
// (matched by email) across ALL inbound configs on ALL servers.
// Returns the list of server IDs that were affected (for sync triggering).
func (s *Service) ResetUserTrafficByEmail(ctx context.Context, email string) ([]int64, error) {
	configs, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var affectedServers []int64
	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}

		settings, ok := inbound["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		configReset := false
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := settings[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				itemEmail, _ := m["email"].(string)
				if itemEmail == "" {
					itemEmail, _ = m["user"].(string)
				}
				// Match exact email or "email_tag" format
				if itemEmail == email || strings.HasPrefix(itemEmail, email+"_") {
					m["_usedUp"] = int64(0)
					m["_usedDown"] = int64(0)
					configReset = true
				}
			}
		}

		if configReset {
			data, err := json.Marshal(inbound)
			if err != nil {
				continue
			}
			cfg.ConfigJSON = string(data)
			cfg.UpdatedAt = time.Now()
			if err := s.repo.Update(ctx, &cfg); err != nil {
				log.Printf("resetUserTraffic: failed to update config %s: %v", cfg.Tag, err)
				continue
			}
			log.Printf("resetUserTraffic: reset traffic for %s in config %s (server %d)", email, cfg.Tag, cfg.ServerID)
			// Track unique server IDs
			found := false
			for _, sid := range affectedServers {
				if sid == cfg.ServerID {
					found = true
					break
				}
			}
			if !found {
				affectedServers = append(affectedServers, cfg.ServerID)
			}
		}
	}

	return affectedServers, nil
}

// GetEnabledInbounds returns all enabled inbound configs as parsed JSON for a specific server.
func (s *Service) GetEnabledInbounds(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error) {
	configs, err := s.repo.List(ctx, serverID, coreType)
	if err != nil {
		return nil, err
	}
	var result []json.RawMessage
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		// Parse the config JSON (contains full inbound object with settings, streamSettings, etc.)
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			return nil, fmt.Errorf("parse inbound %s: %w", cfg.Tag, err)
		}
		// Clean internal fields and filter paused users (shared logic)
		coreconfigSvc.CleanInboundFields(inbound, cfg.Tag, cfg.Listen, cfg.Protocol, cfg.Port)
		data, err := json.Marshal(inbound)
		if err != nil {
			return nil, fmt.Errorf("marshal inbound %s: %w", cfg.Tag, err)
		}
		result = append(result, data)
	}
	return result, nil
}
