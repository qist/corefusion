package system

import (
	"context"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// NodeOperator provides node-level proxy operations for both local and remote nodes.
// For local nodes, the handlers use the proxy/caddy/nginx managers directly.
// For remote nodes, operations are routed via gRPC to the remote Agent.
type NodeOperator interface {
	IsLocalServer(ctx context.Context, serverID int64) bool
	ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error
	SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error
	GetProxyStatusRemote(ctx context.Context, serverID int64) (interface{}, error)
	UpdateProxyGuardian(ctx context.Context, serverID int64, proxyType string, autoStart, autoRestart *bool, maxRestarts *int) error
	InstallProxy(ctx context.Context, serverID int64, proxyType, downloadURL, tagName string) (interface{}, error)
	GetServerPlatform(ctx context.Context, serverID int64) (osName, arch string)
	GetServerDistro(ctx context.Context, serverID int64) string
	// SyncMirrorsProxyConfig pushes proxy config to all mirror nodes of a primary.
	// Used after local proxy config changes (SNI toggle, site enable/disable, etc.)
	// to ensure mirrors get the updated config file and reload.
	SyncMirrorsProxyConfig(ctx context.Context, primaryServerID int64, proxyType string, config []byte) error
	// ControlMirrorsProxy sends a control command (reload/restart) to all mirror
	// nodes of a primary. Used after local proxy reload to trigger mirror reload.
	ControlMirrorsProxy(ctx context.Context, primaryServerID int64, proxyType, action string) error
	// SyncMirrorsData triggers a full database-level data sync (SyncData) to all
	// mirror nodes of a primary. Used after DB-level changes (SNI toggle, port
	// batch update, site enable/disable) to ensure mirrors' local DBs are updated
	// with the primary's latest data.
	SyncMirrorsData(ctx context.Context, primaryServerID int64) error
	// SyncWAFFiles pushes WAF files (config.lua, domain.json, *.rule) to a remote Agent.
	SyncWAFFiles(ctx context.Context, serverID int64, files []*agentpb.WAFFile) error
}
