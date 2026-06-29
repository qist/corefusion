package inboundconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles inbound config business logic.
type Service struct {
	repo *repository.InboundConfigRepository
	db   *gorm.DB
}

// NewService creates a new inbound config service.
func NewService(repo *repository.InboundConfigRepository, db *gorm.DB) *Service {
	return &Service{repo: repo, db: db}
}

// List returns all inbound configs for a core type.
func (s *Service) List(ctx context.Context, coreType string) ([]entity.InboundConfig, error) {
	return s.repo.List(ctx, coreType)
}

// GetByID returns an inbound config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new inbound config.
func (s *Service) Create(ctx context.Context, coreType, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	cfg := &entity.InboundConfig{
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

// UpdateTraffic updates traffic stats for a user by email.
func (s *Service) UpdateTraffic(ctx context.Context, tag string, email string, upDelta int64, downDelta int64) error {
	// Find all inbound configs
	configs, err := s.repo.List(ctx, "")
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

// GetEnabledInbounds returns all enabled inbound configs as parsed JSON.
func (s *Service) GetEnabledInbounds(ctx context.Context, coreType string) ([]json.RawMessage, error) {
	configs, err := s.repo.List(ctx, coreType)
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
		// Ensure required fields are set from the record
		if cfg.Tag != "" {
			inbound["tag"] = cfg.Tag
		}
		if cfg.Listen != "" {
			inbound["listen"] = cfg.Listen
		}
		if cfg.Port > 0 {
			inbound["port"] = cfg.Port
		}
		if cfg.Protocol != "" {
			inbound["protocol"] = cfg.Protocol
		}
		// Remove internal fields
		delete(inbound, "_remark")
		delete(inbound, "_enabled")
		delete(inbound, "_listenType")
		delete(inbound, "_socketPath")
		delete(inbound, "_socketPerm")
		delete(inbound, "_wgPublicKey")
		delete(inbound, "_accessDomain")
		delete(inbound, "_portHop")
		// Clean nested _ fields from users/clients/accounts and filter paused users
		containers := []map[string]interface{}{inbound}
		if settings, ok := inbound["settings"].(map[string]interface{}); ok {
			containers = append(containers, settings)
		}
		for _, c := range containers {
			for _, key := range []string{"users", "clients", "accounts"} {
				if items, ok := c[key].([]interface{}); ok {
					var filtered []interface{}
					for _, item := range items {
						if m, ok := item.(map[string]interface{}); ok {
							// Skip paused users
							if paused, _ := m["_paused"].(bool); paused {
								continue
							}
							delete(m, "_quota")
							delete(m, "_expiry")
							delete(m, "_usedUp")
							delete(m, "_usedDown")
							delete(m, "_paused")
							filtered = append(filtered, m)
						}
					}
					c[key] = filtered
				}
			}
		}
		data, err := json.Marshal(inbound)
		if err != nil {
			return nil, fmt.Errorf("marshal inbound %s: %w", cfg.Tag, err)
		}
		result = append(result, data)
	}
	return result, nil
}
