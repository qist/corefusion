package configsync

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	coreconfigSvc "corefusion/internal/service/coreconfig"
)

// CoreConfigService provides access to core configuration.
type CoreConfigService interface {
	GetFull(ctx context.Context, serverID int64, coreType string) (map[string]interface{}, error)
}

// InboundConfigService provides access to inbound configurations.
type InboundConfigService interface {
	GetEnabledInbounds(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error)
}

// OutboundConfigService provides access to outbound configurations.
type OutboundConfigService interface {
	GetEnabledOutbounds(ctx context.Context, serverID int64, coreType string) ([]map[string]interface{}, error)
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
	// Config sync operates on the local node (server_id=1)
	const localServerID = 1

	// Get full config from core_configs table
	fullConfig, err := s.coreSvc.GetFull(ctx, localServerID, coreType)
	if err != nil {
		return err
	}

	// Use the shared PrepareFullConfig to apply all standard steps
	data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
		Ctx:              ctx,
		ServerID:         localServerID,
		CoreType:         coreType,
		FullConfig:       fullConfig,
		InboundProvider:  s.inboundSvc,
		OutboundProvider: s.outboundSvc,
	})
	if err != nil {
		return err
	}

	// Write to runtime directory
	runtimeDir := filepath.Join(s.dataDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(runtimeDir, coreType+".jsonc")
	return os.WriteFile(configPath, data, 0640)
}
