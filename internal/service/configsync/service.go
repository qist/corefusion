package configsync

import (
	"context"
	"encoding/json"
	"fmt"
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

// ServerLister lists all registered server IDs.
// Implemented by repository.NodeServerRepository.
type ServerLister interface {
	ListAllServerIDs(ctx context.Context) ([]int64, error)
}

// RemoteConfigSyncer syncs config data to remote nodes.
// Implemented by agent.NodeServerManager.
type RemoteConfigSyncer interface {
	SyncConfig(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error
	IsOnline(serverID int64) bool
}

// LogModeProvider provides the log output mode for a core type.
type LogModeProvider interface {
	GetLogMode(ctx context.Context, coreType string) string
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

	// configWriter is an optional delegate for writing runtime config files.
	// When set, configsync uses this instead of direct os.WriteFile.
	// This allows process.Manager.WriteRuntimeConfig (which has a mutex)
	// to protect against concurrent writes from LocalAgent.SyncConfig.
	configWriter ConfigWriter

	// serverLister lists all registered server IDs for multi-node sync.
	serverLister ServerLister

	// remoteSyncer syncs config to remote nodes via Agent.
	remoteSyncer RemoteConfigSyncer

	// logModeProvider provides the log mode for each core type.
	logModeProvider LogModeProvider
}

// ConfigWriter writes runtime config files. Implemented by process.Manager.
type ConfigWriter interface {
	WriteRuntimeConfig(coreType string, data []byte) error
}

// SetConfigWriter sets the config writer delegate for thread-safe config writes.
func (s *Service) SetConfigWriter(w ConfigWriter) {
	s.configWriter = w
}

// SetServerLister sets the server lister for multi-node sync.
func (s *Service) SetServerLister(l ServerLister) {
	s.serverLister = l
}

// SetRemoteSyncer sets the remote config syncer for pushing configs to Agents.
func (s *Service) SetRemoteSyncer(r RemoteConfigSyncer) {
	s.remoteSyncer = r
}

// SetLogModeProvider sets the provider for log output mode per core type.
func (s *Service) SetLogModeProvider(p LogModeProvider) {
	s.logModeProvider = p
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
	// Determine which servers to sync.
	// Always sync local node (server_id=1) to disk.
	// For remote nodes, generate config from panel DB and push via Agent.
	serverIDs := []int64{1} // local node always synced

	if s.serverLister != nil {
		if allIDs, err := s.serverLister.ListAllServerIDs(ctx); err == nil {
			for _, id := range allIDs {
				if id != 1 { // skip local, already added
					serverIDs = append(serverIDs, id)
				}
			}
		}
	}

	var firstErr error
	for _, serverID := range serverIDs {
		if err := s.syncCoreForServer(ctx, coreType, serverID); err != nil {
			log.Printf("configsync: %s sync failed for server %d: %v", coreType, serverID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// syncCoreForServer generates the full config for a specific server and syncs it.
// For server_id=1 (local), writes to disk via configWriter.
// For remote servers, pushes config via Agent gRPC.
func (s *Service) syncCoreForServer(ctx context.Context, coreType string, serverID int64) error {
	// Get full config from core_configs table for this server
	fullConfig, err := s.coreSvc.GetFull(ctx, serverID, coreType)
	if err != nil {
		return err
	}

	// Determine log mode for this core type
	logMode := ""
	if s.logModeProvider != nil {
		logMode = s.logModeProvider.GetLogMode(ctx, coreType)
	}

	// Use the shared PrepareFullConfig to apply all standard steps
	data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
		Ctx:              ctx,
		ServerID:         serverID,
		CoreType:         coreType,
		FullConfig:       fullConfig,
		InboundProvider:  s.inboundSvc,
		OutboundProvider: s.outboundSvc,
		LogMode:          logMode,
	})
	if err != nil {
		return err
	}

	if serverID == 1 {
		// Local node: write to runtime directory
		if s.configWriter != nil {
			// Use the shared config writer (process.Manager.WriteRuntimeConfig)
			// for thread-safe writes that avoid conflicts with LocalAgent.SyncConfig.
			if err := s.configWriter.WriteRuntimeConfig(coreType, data); err != nil {
				return err
			}
		} else {
			// Fallback: direct file write (not thread-safe)
			runtimeDir := filepath.Join(s.dataDir, "runtime")
			if err := os.MkdirAll(runtimeDir, 0755); err != nil {
				return err
			}
			configPath := filepath.Join(runtimeDir, coreType+".jsonc")
			if err := os.WriteFile(configPath, data, 0640); err != nil {
				return err
			}
		}
	} else {
		// Remote node: push config via Agent if online
		if s.remoteSyncer == nil || !s.remoteSyncer.IsOnline(serverID) {
			log.Printf("configsync: server %d offline or remoteSyncer not set, skipping", serverID)
			return nil
		}
		if err := s.remoteSyncer.SyncConfig(ctx, serverID, coreType, data, 0); err != nil {
			return fmt.Errorf("sync to remote server %d: %w", serverID, err)
		}
		log.Printf("configsync: %s config synced to remote server %d (%d bytes)", coreType, serverID, len(data))
	}
	return nil
}
