package configsync

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CoreConfigService provides access to core configuration.
type CoreConfigService interface {
	GetFull(ctx context.Context, coreType string) (map[string]interface{}, error)
}

// InboundConfigService provides access to inbound configurations.
type InboundConfigService interface {
	GetEnabledInbounds(ctx context.Context, coreType string) ([]json.RawMessage, error)
}

// OutboundConfigService provides access to outbound configurations.
type OutboundConfigService interface {
	GetEnabledOutbounds(ctx context.Context, coreType string) ([]map[string]interface{}, error)
}

// Service handles periodic config synchronization to disk.
type Service struct {
	coreSvc     CoreConfigService
	inboundSvc  InboundConfigService
	outboundSvc OutboundConfigService
	dataDir     string
	interval    time.Duration
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
	lastSync    time.Time
}

// NewService creates a new config sync service.
func NewService(
	coreSvc CoreConfigService,
	inboundSvc InboundConfigService,
	outboundSvc OutboundConfigService,
	dataDir string,
	interval time.Duration,
) *Service {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Service{
		coreSvc:     coreSvc,
		inboundSvc:  inboundSvc,
		outboundSvc: outboundSvc,
		dataDir:     dataDir,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the periodic config sync service.
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.loop()
	log.Printf("configsync: started with interval %v", s.interval)
}

// Stop stops the config sync service.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	log.Println("configsync: stopped")
}

// SyncNow triggers an immediate config sync.
func (s *Service) SyncNow(ctx context.Context) error {
	return s.sync(ctx)
}

// LastSync returns the last sync time.
func (s *Service) LastSync() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSync
}

func (s *Service) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Initial sync after a short delay
	time.Sleep(10 * time.Second)
	ctx := context.Background()
	if err := s.sync(ctx); err != nil {
		log.Printf("configsync: initial sync failed: %v", err)
	}

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx := context.Background()
			if err := s.sync(ctx); err != nil {
				log.Printf("configsync: sync failed: %v", err)
			}
		}
	}
}

func (s *Service) sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sync xray config
	if err := s.syncCore(ctx, "xray"); err != nil {
		log.Printf("configsync: xray sync failed: %v", err)
	}

	s.lastSync = time.Now()
	log.Printf("configsync: sync completed at %v", s.lastSync)
	return nil
}

func (s *Service) syncCore(ctx context.Context, coreType string) error {
	// Get full config from core_configs table
	fullConfig, err := s.coreSvc.GetFull(ctx, coreType)
	if err != nil {
		return err
	}

	// Clean empty fields
	cleanEmpty(fullConfig)

	// Remove outbounds from core_configs (migrated to outbound_configs table)
	delete(fullConfig, "outbounds")

	// Ensure stats is present
	if _, exists := fullConfig["stats"]; !exists {
		fullConfig["stats"] = map[string]interface{}{}
	}

	// Handle API config
	// - If api.listen is set: use it directly, no auto-add inbound/routing
	// - If api.listen is empty: auto-add inbound (dokodemo-door) + routing rule
	if apiRaw, ok := fullConfig["api"]; ok {
		if apiMap, ok := apiRaw.(map[string]interface{}); ok {
			// Ensure necessary services
			apiMap["services"] = []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"}

			listen, _ := apiMap["listen"].(string)
			tag, _ := apiMap["tag"].(string)
			if tag == "" {
				tag = "api"
			}

			if listen == "" {
				// listen is empty: auto-add inbound + routing
				delete(apiMap, "listen")

				// Check if API inbound already exists
				inbounds, _ := fullConfig["inbounds"].([]interface{})
				hasAPIInbound := false
				for _, ib := range inbounds {
					if ibMap, ok := ib.(map[string]interface{}); ok {
						if ibMap["tag"] == tag {
							hasAPIInbound = true
							break
						}
					}
				}

				// Add API inbound (dokodemo-door) if not exists
				if !hasAPIInbound {
					apiInbound := map[string]interface{}{
						"tag":      tag,
						"listen":   "127.0.0.1",
						"port":     10085,
						"protocol": "dokodemo-door",
						"settings": map[string]interface{}{"address": "127.0.0.1"},
					}
					fullConfig["inbounds"] = append(inbounds, apiInbound)
				}

				// Add routing rule for API if not exists
				routing, _ := fullConfig["routing"].(map[string]interface{})
				if routing == nil {
					routing = map[string]interface{}{"domainStrategy": "IPIfNonMatch"}
				}
				rules, _ := routing["rules"].([]interface{})
				hasAPIRule := false
				for _, r := range rules {
					if rm, ok := r.(map[string]interface{}); ok {
						if tags, ok := rm["inboundTag"].([]interface{}); ok {
							for _, t := range tags {
								if t == tag {
									hasAPIRule = true
									break
								}
							}
						}
					}
				}
				if !hasAPIRule {
					apiRule := map[string]interface{}{
						"type":        "field",
						"inboundTag":  []string{tag},
						"outboundTag": tag,
					}
					routing["rules"] = append([]interface{}{apiRule}, rules...)
					fullConfig["routing"] = routing
				}
			}
			// else: listen is set, use it directly (no auto-add)
		}
	}

	// Merge enabled inbounds from inbound_configs table
	if s.inboundSvc != nil {
		enabledInbounds, err := s.inboundSvc.GetEnabledInbounds(ctx, coreType)
		if err != nil {
			log.Printf("configsync: failed to load inbounds: %v", err)
		} else if len(enabledInbounds) > 0 {
			existingInbounds, _ := fullConfig["inbounds"].([]interface{})
			for _, ib := range enabledInbounds {
				var inbound map[string]interface{}
				if json.Unmarshal(ib, &inbound) == nil {
					existingInbounds = append(existingInbounds, inbound)
				}
			}
			fullConfig["inbounds"] = existingInbounds
		}
	}

	// Merge enabled outbounds from outbound_configs table
	if s.outboundSvc != nil {
		enabledOutbounds, err := s.outboundSvc.GetEnabledOutbounds(ctx, coreType)
		if err != nil {
			log.Printf("configsync: failed to load outbounds: %v", err)
		} else if len(enabledOutbounds) > 0 {
			existingOutbounds, _ := fullConfig["outbounds"].([]interface{})
			for _, ob := range enabledOutbounds {
				existingOutbounds = append(existingOutbounds, ob)
			}
			fullConfig["outbounds"] = existingOutbounds
		}
	}

	// Write to runtime directory
	runtimeDir := filepath.Join(s.dataDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(runtimeDir, coreType+".jsonc")
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0640)
}

// cleanEmpty removes nil values and empty maps recursively.
func cleanEmpty(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if sub, ok := v.(map[string]interface{}); ok {
			cleanEmpty(sub)
			if len(sub) == 0 {
				delete(m, k)
			}
		}
	}
}
