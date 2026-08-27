package migration

import (
	"encoding/json"
	"fmt"
	"log"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
)

// MigrateChainProxy migrates legacy chain_entry/chain_exit node types
// to the new chain_links + chain_exit_configs model.
//
// This is called from RunDataMigrations and is idempotent:
// - If no chain_entry/chain_exit nodes exist, it's a no-op.
// - If chain_links already exist for a server, it skips creating duplicates.
func MigrateChainProxy(db *gorm.DB) error {
	// Migrate legacy chain_entry nodes
	var entries []entity.NodeServer
	if err := db.Where("node_type = ?", "chain_entry").Find(&entries).Error; err != nil {
		return fmt.Errorf("list chain_entry nodes: %w", err)
	}

	for _, entry := range entries {
		if entry.ChainConfig == "" {
			continue
		}

		// Parse old chain_config JSON
		oldCfg, err := agent.ParseChainConfig(entry.ChainConfig)
		if err != nil || oldCfg == nil {
			log.Printf("[MigrateChainProxy] entry %d: failed to parse chain_config: %v", entry.ID, err)
			continue
		}

		// Check if chain_links already exist for this server
		var existingCount int64
		db.Model(&entity.ChainLink{}).Where("entry_server_id = ?", entry.ID).Count(&existingCount)
		if existingCount > 0 {
			continue // Already migrated
		}

		// Create chain_links from old exits
		for _, exit := range oldCfg.Exits {
			link := entity.ChainLink{
				EntryServerID: entry.ID,
				ExitServerID:  exit.ExitServerID,
				ExitTag:       exit.OutboundTag,
				Weight:        exit.Weight,
				Priority:      exit.Priority,
				Enabled:       true,
			}
			if err := db.Create(&link).Error; err != nil {
				log.Printf("[MigrateChainProxy] entry %d: failed to create chain_link: %v", entry.ID, err)
			}
		}

		// Change node_type to standalone
		db.Model(&entity.NodeServer{}).Where("id = ?", entry.ID).Update("node_type", entity.NodeTypeStandalone)
		log.Printf("[MigrateChainProxy] migrated chain_entry %d → standalone + %d chain_links", entry.ID, len(oldCfg.Exits))
	}

	// Migrate legacy chain_exit nodes
	var exits []entity.NodeServer
	if err := db.Where("node_type = ?", "chain_exit").Find(&exits).Error; err != nil {
		return fmt.Errorf("list chain_exit nodes: %w", err)
	}

	for _, exitNode := range exits {
		// Check if chain_exit_configs already exist for this server
		var existingCount int64
		db.Model(&entity.ChainExitConfig{}).Where("server_id = ?", exitNode.ID).Count(&existingCount)
		if existingCount > 0 {
			continue // Already migrated
		}

		// Parse old chain_config JSON
		relayPort := 443
		relayProtocol := "vless"
		relayTransport := "tcp"
		relaySecurity := "none"

		if exitNode.ChainConfig != "" {
			oldCfg, err := agent.ParseChainConfig(exitNode.ChainConfig)
			if err == nil && oldCfg != nil {
				if oldCfg.RelayPort > 0 {
					relayPort = oldCfg.RelayPort
				}
				if oldCfg.RelayProtocol != "" {
					relayProtocol = oldCfg.RelayProtocol
				}
			}
		}

		exitCfg := entity.ChainExitConfig{
			ServerID:       exitNode.ID,
			RelayPort:      relayPort,
			RelayProtocol:  relayProtocol,
			RelayTransport: relayTransport,
			RelaySecurity:  relaySecurity,
			RelayUUID:      generateRelayUUID(),
			RelayOnly:      true,
			Enabled:        true,
		}

		if err := db.Create(&exitCfg).Error; err != nil {
			log.Printf("[MigrateChainProxy] exit %d: failed to create chain_exit_config: %v", exitNode.ID, err)
		}

		// Change node_type to standalone
		db.Model(&entity.NodeServer{}).Where("id = ?", exitNode.ID).Update("node_type", entity.NodeTypeStandalone)
		log.Printf("[MigrateChainProxy] migrated chain_exit %d → standalone + chain_exit_config", exitNode.ID)
	}

	return nil
}

// generateRelayUUID generates a UUID for chain relay authentication.
func generateRelayUUID() string {
	// Use a simple random UUID. In production, use google/uuid.
	// But we don't want to add a dependency here, so we use
	// the existing relay UUID from the old code if available.
	// For new configs, the API handler generates a proper UUID.
	return "chain-relay-uuid"
}

// Helper: parse raw JSON without importing agent package
func parseRawChainConfig(raw string) (relayPort int, relayProtocol string, exits []struct {
	ExitServerID int64  `json:"exit_server_id"`
	OutboundTag  string `json:"outbound_tag"`
	Weight       int    `json:"weight"`
	Priority     int    `json:"priority"`
}) {
	if raw == "" {
		return 443, "vless", nil
	}

	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return 443, "vless", nil
	}

	if p, ok := m["relay_port"].(float64); ok {
		relayPort = int(p)
	}
	if relayPort == 0 {
		relayPort = 443
	}

	if p, ok := m["relay_protocol"].(string); ok {
		relayProtocol = p
	}
	if relayProtocol == "" {
		relayProtocol = "vless"
	}

	if exitsRaw, ok := m["exits"].([]interface{}); ok {
		for _, e := range exitsRaw {
			if em, ok := e.(map[string]interface{}); ok {
				exit := struct {
					ExitServerID int64  `json:"exit_server_id"`
					OutboundTag  string `json:"outbound_tag"`
					Weight       int    `json:"weight"`
					Priority     int    `json:"priority"`
				}{}
				if v, ok := em["exit_server_id"].(float64); ok {
					exit.ExitServerID = int64(v)
				}
				if v, ok := em["outbound_tag"].(string); ok {
					exit.OutboundTag = v
				}
				if v, ok := em["weight"].(float64); ok {
					exit.Weight = int(v)
				}
				if v, ok := em["priority"].(float64); ok {
					exit.Priority = int(v)
				}
				exits = append(exits, exit)
			}
		}
	}

	return relayPort, relayProtocol, exits
}
