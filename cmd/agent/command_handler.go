package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	agent "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	ratelimit "corefusion/internal/runtime/agent/ratelimit"
	logManageSvc "corefusion/internal/service/logmanage"
	proxySvc "corefusion/internal/service/proxy"
	"corefusion/internal/util/safepath"
	"corefusion/pkg/netutil"
)

// handlePanelCommand dispatches a PanelCommand received via the heartbeat stream
// to the appropriate LocalAgent method. Results are sent back as TaskResult reports.
func (a *AgentApp) handlePanelCommand(cmd *agentpb.PanelCommand, stream agentpb.HeartbeatService_StreamClient) {
	ctx := context.Background()
	cmdID := cmd.CommandId

	// sendSyncResult is a helper for synchronous read operations.
	// It executes the function, marshals the result to detail_json, and sends a TaskResult.
	sendSyncResult := func(taskType string, resp interface{}, execErr error) {
		if execErr != nil {
			a.sendTaskResult(stream, &agentpb.TaskResult{
				TaskId:     cmdID,
				Status:     agentpb.TaskStatus_TASK_FAILED,
				Error:      execErr.Error(),
				TaskType:   taskType,
				FinishedAt: time.Now().Unix(),
			})
			return
		}
		jsonBytes, _ := json.Marshal(resp)
		a.sendTaskResult(stream, &agentpb.TaskResult{
			TaskId:     cmdID,
			Status:     agentpb.TaskStatus_TASK_SUCCESS,
			TaskType:   taskType,
			DetailJson: string(jsonBytes),
			FinishedAt: time.Now().Unix(),
		})
	}

	// submitAsyncTask is a helper for asynchronous operations.
	// It submits to the TaskQueue with a callback that sends TaskResult via stream.
	submitAsyncTask := func(taskType, idempotencyKey string, fn func(ctx context.Context) (interface{}, error)) {
		a.localAgent.GetTaskQueue().Submit(cmdID, taskType, idempotencyKey, fn, func(result *agentpb.TaskResult) {
			if result.TaskId == "" {
				result.TaskId = cmdID
			}
			a.sendTaskResult(stream, result)
		})
		// Immediately send TASK_QUEUED confirmation so the Panel knows the command was accepted
		a.sendTaskResult(stream, &agentpb.TaskResult{
			TaskId:   cmdID,
			Status:   agentpb.TaskStatus_TASK_QUEUED,
			TaskType: taskType,
		})
	}

	switch c := cmd.Command.(type) {

	// ========== Fire-and-forget commands ==========
	case *agentpb.PanelCommand_SyncConfig:
		req := c.SyncConfig
		if err := a.applySyncedConfig(ctx, req.CoreType, req.ConfigContent, req.ConfigVersion); err != nil {
			log.Printf("[Agent] config sync failed: %s: %v", req.CoreType, err)
		} else {
			log.Printf("[Agent] config synced: %s (version=%d)", req.CoreType, req.ConfigVersion)
		}

	case *agentpb.PanelCommand_ControlCore:
		req := c.ControlCore
		if err := a.localAgent.ControlCore(ctx, req.CoreType, req.Action); err != nil {
			log.Printf("[Agent] ControlCore %s %s failed: %v", req.CoreType, req.Action, err)
		} else {
			log.Printf("[Agent] ControlCore %s %s accepted", req.CoreType, req.Action)
		}

	case *agentpb.PanelCommand_SyncProxyConfig:
		req := c.SyncProxyConfig
		// For nginx, ensure stdout/stderr symlinks exist so that the config's
		// access_log/error_log directives referencing logs/stdout_*.log resolve
		// correctly. The panel creates these locally via ensureSymlink, but
		// remote agents need to create them on their own filesystem.
		if req.ProxyType == "nginx" {
			logsDir := filepath.Join(a.dataDir, "logs")
			os.MkdirAll(logsDir, 0755)
			for _, link := range []struct{ name, target string }{
				{"stdout_error.log", "/dev/stderr"},
				{"stdout_access.log", "/dev/stdout"},
				{"stdout_stream.log", "/dev/stdout"},
			} {
				linkPath := filepath.Join(logsDir, link.name)
				if fi, err := os.Lstat(linkPath); err == nil {
					// Already exists — check if it's the correct symlink
					if fi.Mode()&os.ModeSymlink != 0 {
						if existing, err := os.Readlink(linkPath); err == nil && existing == link.target {
							continue // correct symlink already in place
						}
					}
					os.Remove(linkPath) // remove stale file/symlink
				}
				os.Symlink(link.target, linkPath)
			}
		}
		// Land TLS certificates to disk before writing the proxy config,
		// so that cert path references in the config are valid immediately.
		for _, certEntry := range req.Certificates {
			if certEntry.CertContent == "" || certEntry.KeyContent == "" {
				continue
			}
			// 面板下发的证书路径由 Agent 以 root 落盘 → 拒绝上跳（绝对路径仍允许：管理员可能
			// 故意写到 nginx 自己的目录）。见安全审计 v5 H2。
			if err := safepath.RejectTraversal("cert path", certEntry.CertPath); err != nil {
				log.Printf("[Agent] SyncProxyConfig: %v", err)
				continue
			}
			if certEntry.KeyPath != "" {
				if err := safepath.RejectTraversal("key path", certEntry.KeyPath); err != nil {
					log.Printf("[Agent] SyncProxyConfig: %v", err)
					continue
				}
			}
			if certEntry.FullchainPath != "" {
				if err := safepath.RejectTraversal("fullchain path", certEntry.FullchainPath); err != nil {
					log.Printf("[Agent] SyncProxyConfig: %v", err)
					continue
				}
			}
			certPath := certEntry.CertPath
			if !filepath.IsAbs(certPath) {
				certPath = filepath.Join(a.dataDir, certPath)
			}
			os.MkdirAll(filepath.Dir(certPath), 0750)
			if err := os.WriteFile(certPath, []byte(certEntry.CertContent), 0640); err != nil {
				log.Printf("[Agent] SyncProxyConfig: write cert %s: %v", certEntry.Domain, err)
				continue
			}
			keyPath := certEntry.KeyPath
			if !filepath.IsAbs(keyPath) {
				keyPath = filepath.Join(a.dataDir, keyPath)
			}
			os.MkdirAll(filepath.Dir(keyPath), 0750)
			if err := os.WriteFile(keyPath, []byte(certEntry.KeyContent), 0600); err != nil {
				log.Printf("[Agent] SyncProxyConfig: write key %s: %v", certEntry.Domain, err)
				continue
			}
			if certEntry.FullchainContent != "" && certEntry.FullchainPath != "" {
				fullchainPath := certEntry.FullchainPath
				if !filepath.IsAbs(fullchainPath) {
					fullchainPath = filepath.Join(a.dataDir, fullchainPath)
				}
				os.MkdirAll(filepath.Dir(fullchainPath), 0750)
				os.WriteFile(fullchainPath, []byte(certEntry.FullchainContent), 0640)
			}
			log.Printf("[Agent] SyncProxyConfig: certificate landed: %s → %s", certEntry.Domain, certPath)
		}
		// Write config file synchronously (not via task queue) so that any
		// subsequent ControlProxy reload command will see the new config.
		if a.proxyMgr != nil {
			if err := a.proxyMgr.WriteConfigFile(proxySvc.ProxyType(req.ProxyType), string(req.ConfigContent)); err != nil {
				log.Printf("[Agent] proxy config sync failed: %s: %v", req.ProxyType, err)
			} else {
				log.Printf("[Agent] proxy config synced: %s (%d bytes, %d certs)", req.ProxyType, len(req.ConfigContent), len(req.Certificates))
			}
		}
		// Save per-node settings for nginx/caddy to local settings store
		if req.ProxyType == "nginx" && req.NginxSettings != "" {
			if err := a.settingRepo.Set(ctx, "proxy.nginx_settings", req.NginxSettings); err != nil {
				log.Printf("[Agent] failed to save nginx settings: %v", err)
			}
			// 依据面板下发的 base64 素材，在本节点还原 dhparam / 会话票据密钥
			// （配置引用了这些文件，必须在 nginx 加载前就位）。
			var tlsCfg struct {
				DhparamB64 string `json:"ssl_dhparam_b64"`
				TicketB64  string `json:"ssl_session_ticket_b64"`
				TicketBB64 string `json:"ssl_session_ticket_b_b64"`
			}
			if json.Unmarshal([]byte(req.NginxSettings), &tlsCfg) == nil {
				if err := proxySvc.WriteGlobalSslFiles(a.dataDir, tlsCfg.DhparamB64, tlsCfg.TicketB64, tlsCfg.TicketBB64); err != nil {
					log.Printf("[Agent] restore global ssl files: %v", err)
				}
			}
		}
		if req.ProxyType == "caddy" && req.CaddySniSettings != "" {
			// Parse and save caddy SNI settings
			var sniSettings struct {
				Enabled            bool     `json:"enabled"`
				ListenPort         string   `json:"listen_port"`
				LocalPort          string   `json:"local_port"`
				ProxyProtocolRules []string `json:"proxy_protocol_rules"`
			}
			if err := json.Unmarshal([]byte(req.CaddySniSettings), &sniSettings); err == nil {
				enabled := "false"
				if sniSettings.Enabled {
					enabled = "true"
				}
				a.settingRepo.Set(ctx, "proxy.sni_enabled", enabled)
				a.settingRepo.Set(ctx, "proxy.sni_listen_port", sniSettings.ListenPort)
				a.settingRepo.Set(ctx, "proxy.sni_local_port", sniSettings.LocalPort)
				if rules, err := json.Marshal(sniSettings.ProxyProtocolRules); err == nil {
					a.settingRepo.Set(ctx, "proxy.proxy_protocol_rules", string(rules))
				}
			}
		}

	case *agentpb.PanelCommand_SyncWafFiles:
		req := c.SyncWafFiles
		wafDir := filepath.Join(a.dataDir, "proxy", "waf-rules")
		nginxConfigDir := filepath.Join(a.dataDir, "proxy", "lua", "waf")
		// Clean up obsolete rule-config directory from old nginx tar.gz
		oldRuleDir := filepath.Join(nginxConfigDir, "rule-config")
		if _, err := os.Stat(oldRuleDir); err == nil {
			os.RemoveAll(oldRuleDir)
			log.Printf("[Agent] SyncWAFFiles: removed obsolete rule-config directory")
		}
		// Check which proxies are installed so we only write relevant config files.
		nginxInfo := a.proxyMgr.DetectProxy(proxySvc.ProxyNginx)
		caddyInfo := a.proxyMgr.DetectProxy(proxySvc.ProxyCaddy)
		// If neither proxy is installed, skip all file writes — WAF rules
		// are useless without a proxy to enforce them.
		if !nginxInfo.Installed && !caddyInfo.Installed {
			log.Printf("[Agent] SyncWAFFiles: no proxy installed, skipping all file writes")
			break
		}
		os.MkdirAll(wafDir, 0755)
		anyWritten := false
		for _, f := range req.Files {
			// Prevent path traversal
			if strings.Contains(f.Path, "..") {
				continue
			}
			// config.lua is the Nginx WAF config — skip if nginx not installed.
			// config.json is the Caddy WAF config — skip if caddy not installed.
			// Rule files (.rule, domain.json, domain-specific rules) are shared
			// and always written regardless of which proxy is installed.
			if f.Path == "config.lua" && !nginxInfo.Installed {
				continue
			}
			if f.Path == "config.json" && !caddyInfo.Installed {
				continue
			}
			// config.lua goes to proxy/lua/waf/config.lua (Lua engine loads it there)
			// All other files go to proxy/waf-rules/
			var outPath string
			if f.Path == "config.lua" {
				outPath = filepath.Join(nginxConfigDir, "config.lua")
			} else {
				outPath = filepath.Join(wafDir, filepath.FromSlash(f.Path))
			}
			// Hash comparison: skip if content unchanged
			if f.Hash != "" {
				if existing, err := os.ReadFile(outPath); err == nil {
					h := sha256.Sum256(existing)
					if hex.EncodeToString(h[:]) == f.Hash {
						continue
					}
				}
			}
			os.MkdirAll(filepath.Dir(outPath), 0755)
			if err := os.WriteFile(outPath, f.Content, 0644); err != nil {
				log.Printf("[Agent] SyncWAFFiles: write %s: %v", f.Path, err)
			} else {
				log.Printf("[Agent] SyncWAFFiles: wrote %s (%d bytes)", f.Path, len(f.Content))
				anyWritten = true
			}
		}
		// Reload proxies so WAF changes take effect immediately.
		// Only reload proxies that are installed and running — skip
		// uninstalled or stopped proxies to avoid triggering auto-start.
		if anyWritten {
			for _, pt := range []string{"nginx", "caddy"} {
				info := a.proxyMgr.DetectProxy(proxySvc.ProxyType(pt))
				if !info.Installed || info.Status != "running" {
					continue
				}
				if err := a.localAgent.ControlProxy(ctx, pt, "reload"); err != nil {
					log.Printf("[Agent] SyncWAFFiles: reload %s: %v", pt, err)
				}
			}
		}

	case *agentpb.PanelCommand_ControlProxy:
		req := c.ControlProxy
		if err := a.localAgent.ControlProxy(ctx, req.ProxyType, req.Action); err != nil {
			log.Printf("[Agent] ControlProxy %s %s failed: %v", req.ProxyType, req.Action, err)
		} else {
			log.Printf("[Agent] ControlProxy %s %s accepted", req.ProxyType, req.Action)
		}

	case *agentpb.PanelCommand_AlterUsers:
		req := c.AlterUsers
		if err := a.localAgent.AlterUsers(ctx, req); err != nil {
			log.Printf("[Agent] AlterUsers %s failed: %v", req.Action, err)
		} else {
			log.Printf("[Agent] AlterUsers %s accepted", req.Action)
		}

	case *agentpb.PanelCommand_RenewCert:
		log.Printf("[Agent] renewing mTLS certificate (expires %v)", time.Unix(c.RenewCert.ExpiresAt, 0))
		if err := os.WriteFile(a.certPath, c.RenewCert.NewCertPem, 0640); err != nil {
			log.Printf("[Agent] write renewed cert: %v", err)
		} else if err := os.WriteFile(a.keyPath, c.RenewCert.NewKeyPem, 0600); err != nil {
			log.Printf("[Agent] write renewed key: %v", err)
		} else {
			log.Println("[Agent] mTLS certificate renewed successfully")
		}

	case *agentpb.PanelCommand_IssuePublicCert:
		// 面板下发：在**本机**签发/续签公网证书（域名或 IP ✓），签完回传 PEM 给面板落库 ✓
		log.Printf("[Agent] IssuePublicCert received: cert_id=%d identifiers=%v", c.IssuePublicCert.GetCertId(), c.IssuePublicCert.GetIdentifiers())
		go a.handleIssuePublicCert(c.IssuePublicCert, func(r *agentpb.AgentReport) {
			a.streamSendMu.Lock()
			defer a.streamSendMu.Unlock()
			if err := stream.Send(r); err != nil {
				log.Printf("[Agent] failed to send public cert report: %v", err)
			}
		})

	case *agentpb.PanelCommand_DetectPublicIp:
		// 面板要用**节点自己的公网 IP** 来签 IP 证书 ✓ → 让节点在**本机直连**探测 ✓。
		// ⚠ 面板不能代探测 ✗：面板（尤其在配了出网代理时）出网看到的是**面板/代理的出口 IP** ✗，
		// 而证书 SAN 要写的是该节点对外提供服务的真实 IP ✓（见 netutil.DetectPublicIPDirect ✓）。
		ipv4, ipv6, derr := netutil.DetectPublicIPDirect(ctx, 8*time.Second)
		if derr != nil {
			sendSyncResult("detect_public_ip", nil, derr)
		} else {
			sendSyncResult("detect_public_ip", &agentpb.DetectPublicIPResponse{
				Ipv4: ipv4,
				Ipv6: ipv6,
			}, nil)
		}

	case *agentpb.PanelCommand_DistributeCert:
		a.handleDistributeCert(c.DistributeCert)

	case *agentpb.PanelCommand_GetStatus:
		a.sendStatusReport(stream)

	case *agentpb.PanelCommand_UpdateAgent:
		log.Printf("[Agent] UpdateAgent command received: version=%s", c.UpdateAgent.Version)
		go func() {
			updater := agent.NewSelfUpdater(a.dataDir, a.settingRepo)
			// Resolve relative download URL using the public HTTP endpoint
			// (CDN when configured) from config.
			downloadURL := c.UpdateAgent.DownloadUrl
			if strings.HasPrefix(downloadURL, "/") {
				downloadURL = a.panelHTTPBaseURL() + downloadURL
				log.Printf("[Agent] resolved relative download URL to %s", downloadURL)
			}
			err := updater.PerformUpdate(
				c.UpdateAgent.Version,
				downloadURL,
				c.UpdateAgent.Checksum,
			)
			if err != nil {
				log.Printf("[Agent] update failed: %v", err)
				a.sendTaskResult(stream, &agentpb.TaskResult{
					TaskId:     cmdID,
					Status:     agentpb.TaskStatus_TASK_FAILED,
					Error:      err.Error(),
					TaskType:   "update_agent",
					FinishedAt: time.Now().Unix(),
				})
			} else {
				log.Printf("[Agent] update succeeded, restarting...")
				a.sendTaskResult(stream, &agentpb.TaskResult{
					TaskId:     cmdID,
					Status:     agentpb.TaskStatus_TASK_SUCCESS,
					TaskType:   "update_agent",
					DetailJson: fmt.Sprintf(`{"to_version":"%s"}`, c.UpdateAgent.Version),
					FinishedAt: time.Now().Unix(),
				})
				// restart happens inside PerformUpdate
			}
		}()

	// ========== SyncData — send TaskResult with response ==========
	case *agentpb.PanelCommand_SyncData:
		req := c.SyncData
		log.Printf("[Agent] received SyncData: server_id=%d, %d core_configs, %d inbound_configs, %d outbound_configs, %d warp_configs, %d share_domain_configs",
			req.ServerId, len(req.CoreConfigs), len(req.InboundConfigs), len(req.OutboundConfigs), len(req.WarpConfigs), len(req.ShareDomainConfigs))
		resp, err := a.handleSyncData(ctx, req)
		if err != nil {
			log.Printf("[Agent] SyncData failed: %v", err)
			a.sendTaskResult(stream, &agentpb.TaskResult{
				TaskId:     cmdID,
				Status:     agentpb.TaskStatus_TASK_FAILED,
				Error:      err.Error(),
				TaskType:   "sync_data",
				FinishedAt: time.Now().Unix(),
			})
		} else {
			log.Printf("[Agent] SyncData completed: success=%v, core_configs=%d, inbound=%d, outbound=%d, config_generated=%v, core_started=%v",
				resp.Success, resp.CoreConfigsApplied, resp.InboundConfigsApplied, resp.OutboundConfigsApplied, resp.ConfigGenerated, resp.CoreStarted)
			jsonBytes, _ := json.Marshal(resp)
			a.sendTaskResult(stream, &agentpb.TaskResult{
				TaskId:     cmdID,
				Status:     agentpb.TaskStatus_TASK_SUCCESS,
				TaskType:   "sync_data",
				DetailJson: string(jsonBytes),
				FinishedAt: time.Now().Unix(),
			})
		}

	// ========== Async tasks: submit to TaskQueue, callback sends TaskResult ==========
	case *agentpb.PanelCommand_InstallBinary:
		req := c.InstallBinary
		a.syncGithubAccelSettings(ctx, req.AccelDomain, req.ProxyUrl)
		submitAsyncTask("install_binary",
			fmt.Sprintf("install_binary:%s:%s", req.CoreType, req.Version),
			func(ctx context.Context) (interface{}, error) {
				err := a.localAgent.InstallBinary(ctx, req.CoreType, req.Version, req.DownloadUrl, req.AssetName)
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"core_type": req.CoreType, "version": req.Version}, nil
			})

	case *agentpb.PanelCommand_InstallProxy:
		req := c.InstallProxy
		a.syncGithubAccelSettings(ctx, req.AccelDomain, req.ProxyUrl)
		submitAsyncTask("install_proxy",
			fmt.Sprintf("install_proxy:%s:%s", req.ProxyType, req.TagName),
			func(ctx context.Context) (interface{}, error) {
				if req.DownloadUrl != "" {
					return nil, a.localAgent.InstallProxyVersion(ctx, req.ProxyType, req.DownloadUrl, req.TagName)
				}
				return nil, a.localAgent.InstallProxy(ctx, req.ProxyType)
			})

	// ========== Synchronous read operations: execute and send TaskResult ==========
	case *agentpb.PanelCommand_GetMetrics:
		m := getSystemMetrics()
		resp := &agentpb.GetMetricsResponse{
			Points: []*agentpb.MetricPoint{{
				Timestamp:         time.Now().Unix(),
				CpuUsage:          m.CPUUsage,
				CpuCores:          int32(m.CPUCores),
				MemUsageBytes:     m.MemUsedBytes,
				MemTotalBytes:     m.MemTotalBytes,
				MemUsage:          m.MemUsage,
				SwapTotalBytes:    m.SwapTotalBytes,
				SwapUsedBytes:     m.SwapUsedBytes,
				SwapUsage:         m.SwapUsage,
				DiskUsageBytes:    m.DiskUsedBytes,
				DiskTotalBytes:    m.DiskTotalBytes,
				DiskUsage:         m.DiskUsage,
				DiskReadBytesSec:  m.DiskReadBytesSec,
				DiskWriteBytesSec: m.DiskWriteBytesSec,
				NetRxBytes:        m.NetRxBytesTotal,
				NetTxBytes:        m.NetTxBytesTotal,
				NetRxBytesSec:     m.NetRxBytesSec,
				NetTxBytesSec:     m.NetTxBytesSec,
				Load_1M:           m.Load1m,
				TcpConnections:    int32(m.TCPConnections),
				UptimeSeconds:     m.UptimeSeconds,
			}},
		}
		sendSyncResult("get_metrics", resp, nil)

	case *agentpb.PanelCommand_ListBinaries:
		resp := a.localAgent.ListBinaries()
		sendSyncResult("list_binaries", resp, nil)

	case *agentpb.PanelCommand_GetProxyStatus:
		resp := a.localAgent.GetProxyStatuses()
		sendSyncResult("get_proxy_status", resp, nil)

	case *agentpb.PanelCommand_SpeedTest:
		req := c.SpeedTest
		resp := a.handleSpeedTest(ctx, req)
		sendSyncResult("speed_test", resp, nil)

	case *agentpb.PanelCommand_DownloadGeoRule:
		req := c.DownloadGeoRule
		resp := a.handleDownloadGeoRule(ctx, req)
		sendSyncResult("download_geo_rule", resp, nil)

	case *agentpb.PanelCommand_DeleteGeoRule:
		req := c.DeleteGeoRule
		resp := a.handleDeleteGeoRule(req)
		sendSyncResult("delete_geo_rule", resp, nil)

	case *agentpb.PanelCommand_ListInterfaces:
		names, err := a.localAgent.ListInterfaces()
		sendSyncResult("list_interfaces", &agentpb.ListInterfacesResponse{Interfaces: names}, err)

	case *agentpb.PanelCommand_SetEgressInterface:
		req := c.SetEgressInterface
		SetEgressInterface(req.InterfaceName)
		log.Printf("[Agent] egress interface set to: %q", req.InterfaceName)
		sendSyncResult("set_egress_interface", map[string]interface{}{
			"interface": req.InterfaceName,
			"success":   true,
		}, nil)

	case *agentpb.PanelCommand_ExecuteBinary:
		req := c.ExecuteBinary
		if req.RawOutput {
			out, err := a.localAgent.ExecuteBinaryRawOutput(ctx, req.CoreType, req.Args...)
			sendSyncResult("execute_binary", &agentpb.ExecuteBinaryResponse{
				Success: true, Stdout: out,
			}, err)
		} else {
			parsed, err := a.localAgent.ExecuteBinaryCommand(ctx, req.CoreType, req.Args...)
			sendSyncResult("execute_binary", &agentpb.ExecuteBinaryResponse{
				Success: true, Parsed: parsed,
			}, err)
		}

	case *agentpb.PanelCommand_XrayApi:
		resp, err := a.localAgent.XrayAPIProxy(ctx, c.XrayApi)
		sendSyncResult("xray_api", resp, err)

	case *agentpb.PanelCommand_ReadConfigFile:
		req := c.ReadConfigFile
		data, filePath, err := a.localAgent.ReadConfigFile(req.CoreType)
		sendSyncResult("read_config_file", &agentpb.ReadConfigFileResponse{
			Success: true, Content: data, FilePath: filePath,
		}, err)

	case *agentpb.PanelCommand_ReadLogFile:
		req := c.ReadLogFile
		resp := a.handleReadLogFile(req)
		sendSyncResult("read_log_file", resp, nil)

	case *agentpb.PanelCommand_RotateLog:
		req := c.RotateLog
		resp := a.handleRotateLog(req)
		sendSyncResult("rotate_log", resp, nil)

	case *agentpb.PanelCommand_PackageLogs:
		req := c.PackageLogs
		resp := a.handlePackageLogs(req)
		sendSyncResult("package_logs", resp, nil)

	case *agentpb.PanelCommand_SyncLogConfig:
		req := c.SyncLogConfig
		var syncErr error
		// Write log output configs to local settings
		if req.CoreLogConfig != "" {
			if err := a.settingRepo.Set(ctx, "core.log_output_config", req.CoreLogConfig); err != nil {
				syncErr = fmt.Errorf("write core log config: %w", err)
			}
		}
		if req.ProxyLogConfig != "" {
			if err := a.settingRepo.Set(ctx, "proxy.log_output_config", req.ProxyLogConfig); err != nil {
				syncErr = fmt.Errorf("write proxy log config: %w", err)
			}
		}
		// Write nginx settings to local settings (for log path discovery)
		if req.NginxSettings != "" {
			if err := a.settingRepo.Set(ctx, "proxy.nginx_settings", req.NginxSettings); err != nil {
				syncErr = fmt.Errorf("write nginx settings: %w", err)
			}
		}
		// Start/stop LogWriters based on the new modes (same as Panel's SaveLogOutput/SaveCoreLogOutput)
		if a.logMgr != nil && syncErr == nil {
			// Parse proxy config and start/stop writers for nginx/caddy
			if req.ProxyLogConfig != "" {
				var proxyCfg struct {
					Nginx struct {
						Mode    string `json:"mode"`
						Managed bool   `json:"managed"`
					} `json:"nginx"`
					Caddy struct {
						Mode    string `json:"mode"`
						Managed bool   `json:"managed"`
					} `json:"caddy"`
				}
				if err := json.Unmarshal([]byte(req.ProxyLogConfig), &proxyCfg); err == nil {
					if err := a.logMgr.StartWriterManaged("nginx", proxyCfg.Nginx.Mode, proxyCfg.Nginx.Managed); err != nil {
						log.Printf("[Agent] failed to start nginx writer: %v", err)
					}
					if err := a.logMgr.StartWriterManaged("caddy", proxyCfg.Caddy.Mode, proxyCfg.Caddy.Managed); err != nil {
						log.Printf("[Agent] failed to start caddy writer: %v", err)
					}
				}
			}
			// Parse core config and start/stop writers for xray/singbox
			if req.CoreLogConfig != "" {
				var coreCfg struct {
					Xray struct {
						Mode    string `json:"mode"`
						Managed bool   `json:"managed"`
					} `json:"xray"`
					Singbox struct {
						Mode    string `json:"mode"`
						Managed bool   `json:"managed"`
					} `json:"singbox"`
				}
				if err := json.Unmarshal([]byte(req.CoreLogConfig), &coreCfg); err == nil {
					if err := a.logMgr.StartWriterManaged("xray", coreCfg.Xray.Mode, coreCfg.Xray.Managed); err != nil {
						log.Printf("[Agent] failed to start xray writer: %v", err)
					}
					if err := a.logMgr.StartWriterManaged("singbox", coreCfg.Singbox.Mode, coreCfg.Singbox.Managed); err != nil {
						log.Printf("[Agent] failed to start singbox writer: %v", err)
					}
				}
			}
			log.Printf("[Agent] log writers refreshed after SyncLogConfig")
		}
		sendSyncResult("sync_log_config", map[string]interface{}{
			"success": syncErr == nil,
		}, syncErr)

	case *agentpb.PanelCommand_DiscoverLogs:
		resp := a.handleDiscoverLogs(ctx)
		sendSyncResult("discover_logs", resp, nil)

	case *agentpb.PanelCommand_GetWriterStatus:
		resp := a.handleGetWriterStatus(ctx)
		sendSyncResult("get_writer_status", resp, nil)

	case *agentpb.PanelCommand_StartWriter:
		req := c.StartWriter
		err := a.logMgr.StartWriter(req.ProxyType, req.Mode)
		sendSyncResult("start_writer", map[string]interface{}{
			"success": err == nil,
			"message": fmt.Sprintf("writer %s started mode=%s", req.ProxyType, req.Mode),
		}, err)

	case *agentpb.PanelCommand_StopWriter:
		req := c.StopWriter
		err := a.logMgr.StopWriter(req.ProxyType)
		sendSyncResult("stop_writer", map[string]interface{}{
			"success": err == nil,
			"message": fmt.Sprintf("writer %s stopped", req.ProxyType),
		}, err)

	case *agentpb.PanelCommand_UpdateWriterConfig:
		req := c.UpdateWriterConfig
		cfg := logManageSvc.LogWriterConfig{
			ProxyType:  req.ProxyType,
			Mode:       req.Mode,
			RetainDays: int(req.RetainDays),
			Compress:   req.Compress,
			MaxSizeMB:  int(req.MaxSizeMb),
		}
		err := a.logMgr.UpdateWriterConfig(req.ProxyType, cfg)
		sendSyncResult("update_writer_config", map[string]interface{}{
			"success": err == nil,
			"message": fmt.Sprintf("writer %s config updated", req.ProxyType),
		}, err)

	// ========== Rate Limit commands ==========
	case *agentpb.PanelCommand_AssociateIp:
		req := c.AssociateIp
		submitAsyncTask("associate_ip",
			fmt.Sprintf("associate_ip:%s:%d", req.Ip, req.SubscriptionId),
			func(ctx context.Context) (interface{}, error) {
				if err := a.localAgent.AssociateIP(ctx, req); err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"ip":              req.Ip,
					"subscription_id": req.SubscriptionId,
				}, nil
			})

	case *agentpb.PanelCommand_BatchApplyLimits:
		req := c.BatchApplyLimits
		submitAsyncTask("batch_apply_limits",
			fmt.Sprintf("batch_apply_limits:%d_rules", len(req.Limits)),
			func(ctx context.Context) (interface{}, error) {
				mgr := a.localAgent.GetRateLimitManager()
				if mgr == nil {
					return nil, fmt.Errorf("rate limit manager not configured")
				}
				if req.ResetAll {
					if err := mgr.FlushAll(ctx); err != nil {
						log.Printf("[Agent] BatchApplyLimits reset_all failed: %v", err)
					}
				}
				applied := 0
				for _, limit := range req.Limits {
					switch limit.Action {
					case "apply":
						rate := &ratelimit.RateConfig{
							MaxDownMbps: limit.MaxDownMbps,
							MaxUpMbps:   limit.MaxUpMbps,
							ConnLimit:   limit.ConnLimit,
							DeviceLimit: limit.DeviceLimit,
						}
						if err := mgr.AssociateIP(ctx, limit.Ip, limit.SubscriptionId, rate); err != nil {
							log.Printf("[Agent] BatchApplyLimits apply ip=%s sub=%d failed: %v", limit.Ip, limit.SubscriptionId, err)
							continue
						}
						applied++
					case "remove":
						if err := mgr.RemoveIP(ctx, limit.Ip); err != nil {
							log.Printf("[Agent] BatchApplyLimits remove ip=%s failed: %v", limit.Ip, err)
						}
						applied++
					case "remove_subscription":
						if err := mgr.RemoveSubscription(ctx, limit.SubscriptionId); err != nil {
							log.Printf("[Agent] BatchApplyLimits remove_subscription sub=%d failed: %v", limit.SubscriptionId, err)
						}
						applied++
					}
				}
				return map[string]interface{}{
					"applied": applied,
					"total":   len(req.Limits),
				}, nil
			})

	case *agentpb.PanelCommand_GetRateLimitStatus:
		req := c.GetRateLimitStatus
		mgr := a.localAgent.GetRateLimitManager()
		if mgr == nil {
			// 管理器未初始化（限速从未启用）：仍然返回**真实能力探测**结果，
			// UI 才能显示"该节点支持/不支持限流"，而不是永远 false。
			// 注意 err 传 nil —— 传 err 会走 TASK_FAILED，面板拿不到 DetailJson。
			avail := ratelimit.DetectAvailability()
			sendSyncResult("get_rate_limit_status", map[string]interface{}{
				"entries":             []interface{}{},
				"nft_available":       avail.NftAvailable,
				"tc_available":        avail.TcAvailable,
				"ifb_available":       avail.IfbAvailable,
				"conntrack_available": avail.ConnTrackAvailable,
				"kernel_supported":    avail.KernelSupported,
				"unsupported_reason":  avail.UnsupportedReason,
			}, nil)
			break
		}
		entries := mgr.GetStatus(req.SubscriptionId)
		avail := mgr.IsAvailable()
		type entryJSON struct {
			SubscriptionID int64  `json:"subscription_id"`
			IP             string `json:"ip"`
			Mark           uint32 `json:"mark"`
			MaxDownMbps    int32  `json:"max_down_mbps"`
			MaxUpMbps      int32  `json:"max_up_mbps"`
			DownlinkBytes  int64  `json:"downlink_bytes"`
			UplinkBytes    int64  `json:"uplink_bytes"`
			LastSeen       int64  `json:"last_seen"`
		}
		jsonEntries := make([]entryJSON, 0, len(entries))
		for _, e := range entries {
			jsonEntries = append(jsonEntries, entryJSON{
				SubscriptionID: e.SubscriptionID,
				IP:             e.IP,
				Mark:           e.Mark,
				MaxDownMbps:    e.MaxDownMbps,
				MaxUpMbps:      e.MaxUpMbps,
				DownlinkBytes:  e.DownlinkBytes,
				UplinkBytes:    e.UplinkBytes,
				LastSeen:       e.LastSeen,
			})
		}
		sendSyncResult("get_rate_limit_status", map[string]interface{}{
			"entries":             jsonEntries,
			"nft_available":       avail.NftAvailable,
			"tc_available":        avail.TcAvailable,
			"ifb_available":       avail.IfbAvailable,
			"conntrack_available": avail.ConnTrackAvailable,
			"kernel_supported":    avail.KernelSupported,
			"unsupported_reason":  avail.UnsupportedReason,
		}, nil)

	case *agentpb.PanelCommand_ApplyLimits:
		req := c.ApplyLimits
		// 配置级 apply（不带 IP）= 面板开启/更新限速、或 Agent 重连时补推配置。
		// 这条**必须**走 LocalAgent.ApplyLimits → AgentApp.initRateLimitOnDemand：
		// 落库 + 初始化 nft/tc 后端。以前这里不分青红皂白直接 mgr.AssociateIP(ip="")，
		// 而 Manager 只在 Agent 启动时 Init 过 → 面板开了限速，节点上 nft/tc/IFB 一个都没建
		// （实测 192.168.2.50：journal 只有 `TASK_FAILED err=rate limit manager not initialized`）。
		if req.Action == "apply" && req.Ip == "" {
			submitAsyncTask("apply_limits",
				fmt.Sprintf("apply_limits:config:%d", a.cfg.Panel.ServerID),
				func(ctx context.Context) (interface{}, error) {
					return nil, a.localAgent.ApplyLimits(ctx, req)
				})
			break
		}
		mgr := a.localAgent.GetRateLimitManager()
		if mgr == nil {
			// Fallback: old LimitApplier path
			submitAsyncTask("apply_limits",
				fmt.Sprintf("apply_limits:%s:%d", req.Action, req.SubscriptionId),
				func(ctx context.Context) (interface{}, error) {
					return nil, a.localAgent.ApplyLimits(ctx, req)
				})
			break
		}
		submitAsyncTask("apply_limits",
			fmt.Sprintf("apply_limits:%s:%d", req.Action, req.SubscriptionId),
			func(ctx context.Context) (interface{}, error) {
				rate := &ratelimit.RateConfig{
					MaxDownMbps: req.MaxDownMbps,
					MaxUpMbps:   req.MaxUpMbps,
					ConnLimit:   req.ConnLimit,
					DeviceLimit: req.DeviceLimit,
				}
				// If value is set but specific mbps fields are 0, use value as both
				if req.Value > 0 && rate.MaxDownMbps == 0 && rate.MaxUpMbps == 0 {
					rate.MaxDownMbps = req.Value
					rate.MaxUpMbps = req.Value
				}
				switch req.Action {
				case "apply":
					if err := mgr.AssociateIP(ctx, req.Ip, req.SubscriptionId, rate); err != nil {
						return nil, err
					}
				case "update":
					// 管理端修改客户限速后"立即生效"推送：更新 tc class 速率
					// （0=不限 → 重置为默认 class）并重建该订阅全部已关联 IP 的 connlimit。
					if err := mgr.UpdateRate(ctx, req.SubscriptionId, rate); err != nil {
						return nil, err
					}
				case "remove":
					if err := mgr.RemoveIP(ctx, req.Ip); err != nil {
						return nil, err
					}
				case "remove_subscription":
					if err := mgr.RemoveSubscription(ctx, req.SubscriptionId); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unknown action: %s", req.Action)
				}
				return map[string]interface{}{
					"action":          req.Action,
					"ip":              req.Ip,
					"subscription_id": req.SubscriptionId,
				}, nil
			})

	case *agentpb.PanelCommand_SetIpBlacklist:
		req := c.SetIpBlacklist
		// 同步执行（nft 命令毫秒级）：面板用 SendCommandAndWait 等结果，
		// 便于逐节点记录下发成败。空 entries = 清空黑名单。
		sendSyncResult("set_ip_blacklist", map[string]interface{}{
			"entries": len(req.Entries),
		}, a.localAgent.SetIPBlacklist(ctx, req.Entries))

	default:
		log.Printf("[Agent] unknown command: %T", cmd.Command)
	}
}

// handleSpeedTest backs the remote-node latency probe used by the panel's load
// balancer / latency-based sorting (SpeedTestOnServer → stream command → this handler).
//
// It intentionally does NOT open a TCP connection to any port: the panel measures the
// round-trip time of this gRPC command over the existing heartbeat stream (the same
// channel the Agent already uses to talk to the panel), so the reported latency is the
// real panel↔node control-plane RTT. Probing an arbitrary port (e.g. :443) is both
// redundant and unreliable — nothing is guaranteed to be listening there. The Agent
// therefore just echoes back immediately; the panel fills in LatencyMs from the
// measured round-trip. Bandwidth-related fields stay 0 (this is a lightweight probe,
// not a throughput test).
func (a *AgentApp) handleSpeedTest(ctx context.Context, req *agentpb.SpeedTestRequest) *agentpb.SpeedTestResponse {
	// Echo a tiny bit of processing so the round-trip is a real RPC, not a no-op.
	// No network probe — the panel owns latency measurement.
	samples := int(req.GetSamples())
	if samples <= 0 {
		samples = 1
	}
	select {
	case <-ctx.Done():
		return &agentpb.SpeedTestResponse{Success: false, Error: "context cancelled"}
	case <-time.After(time.Duration(samples) * time.Millisecond):
	}
	return &agentpb.SpeedTestResponse{
		Success:    true,
		LatencyMs:  0,
		JitterMs:   0,
		PacketLoss: 0,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// sendTaskResult sends a TaskResult back to the panel via the heartbeat stream.
// Thread-safe: uses streamSendMu to protect concurrent stream.Send calls.
func (a *AgentApp) sendTaskResult(stream agentpb.HeartbeatService_StreamClient, result *agentpb.TaskResult) {
	report := &agentpb.AgentReport{
		Type:       agentpb.AgentReport_TASK_RESULT,
		ServerId:   a.cfg.Panel.ServerID,
		Timestamp:  time.Now().Unix(),
		TaskResult: result,
	}
	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()
	if err := stream.Send(report); err != nil {
		log.Printf("[Agent] failed to send task result (id=%s type=%s): %v", result.TaskId, result.TaskType, err)
	}
}

// handleDistributeCert writes a distributed certificate to disk at the specified path.
func (a *AgentApp) handleDistributeCert(cmd *agentpb.DistributeCertCommand) {
	if cmd.TargetPath == "" {
		log.Printf("[Agent] DistributeCert: no target_path for %s", cmd.CertName)
		return
	}
	// Resolve relative paths against the Agent's dataDir so that xray config
	// references (relative to data/runtime/) resolve to dataDir/certs/...
	// 路径由面板下发但 Agent 以 root 运行 → 拒绝上跳（管理员配置的绝对路径仍然允许 ✗→✓ 见 safepath）
	if err := safepath.RejectTraversal("distribute cert target path", cmd.TargetPath); err != nil {
		log.Printf("[Agent] DistributeCert: %v", err)
		return
	}
	targetPath := cmd.TargetPath
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(a.dataDir, targetPath)
	}
	// ⚠ `TargetPath` 的语义是"**这张证书的目录**"（面板发的是 `certs/<域名>` ✓）。
	// 以前这里把它当成"证书文件路径" ✗ → 把 PEM 写成了一个**名为域名的文件** ✗、
	// 并把 `key.pem` 写到**上一级**（`certs/key.pem` ✗）→ 内核按
	// `certs/<域名>/fullchain.pem` 去读就报 **`not a directory`** ✗ ——
	// 真机上 192.168.2.180 的 sing-box / nginx 起不来就是这个原因 ✓
	//（节点 `certs/` 根目录那些游离的 `key.pem`/`fullchain.pem` 也是它的产物 ✓）。
	//
	// 兼容 ✓：若 TargetPath 以 `.pem` 结尾（老面板按"文件路径"下发 ✓），仍按文件写 ✓。
	if strings.HasSuffix(strings.ToLower(targetPath), ".pem") {
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			log.Printf("[Agent] distribute cert mkdir error: %v", err)
			return
		}
		if err := os.WriteFile(targetPath, cmd.CertPem, 0640); err != nil {
			log.Printf("[Agent] distribute cert write error: %v", err)
			return
		}
		os.WriteFile(filepath.Join(dir, "key.pem"), cmd.KeyPem, 0600)
		if len(cmd.FullchainPem) > 0 {
			os.WriteFile(filepath.Join(dir, "fullchain.pem"), cmd.FullchainPem, 0640)
		}
		log.Printf("[Agent] certificate %s distributed to %s (file mode)", cmd.CertName, targetPath)
	} else {
		// **自愈** ✓：老版本误写成的"同名文件"会挡住建目录 ✗ → 先删掉它 ✓，
		// 这样升级后**无需人工清理**，下次分发就把正确的目录结构建起来 ✓。
		if st, serr := os.Stat(targetPath); serr == nil && !st.IsDir() {
			if rerr := os.Remove(targetPath); rerr != nil {
				log.Printf("[Agent] DistributeCert: 删除旧版误写的同名文件失败（%s）：%v", targetPath, rerr)
				return
			}
			log.Printf("[Agent] DistributeCert: 已移除旧版误写成的同名文件 %s（它本应是目录 ✓）", targetPath)
		}
		if err := os.MkdirAll(targetPath, 0750); err != nil {
			log.Printf("[Agent] distribute cert mkdir error: %v", err)
			return
		}
		chain := cmd.FullchainPem
		if len(chain) == 0 {
			chain = cmd.CertPem // 没给 fullchain 时退回证书本身 ✓（它本身就含链 ✓）
		}
		if err := os.WriteFile(filepath.Join(targetPath, "cert.pem"), cmd.CertPem, 0640); err != nil {
			log.Printf("[Agent] distribute cert write error: %v", err)
			return
		}
		os.WriteFile(filepath.Join(targetPath, "key.pem"), cmd.KeyPem, 0600)
		os.WriteFile(filepath.Join(targetPath, "fullchain.pem"), chain, 0640)
		log.Printf("[Agent] certificate %s distributed to %s ✓（cert.pem/key.pem/fullchain.pem ✓）", cmd.CertName, targetPath)
	}

	if cmd.ReloadCore {
		log.Printf("[Agent] reload core requested after cert distribution")
		// Core reload would be handled by process.Manager.Restart or Guardian
	}
}

// handleDownloadGeoRule downloads a geo rule file to the Agent's local data/rules/ directory.
func (a *AgentApp) handleDownloadGeoRule(ctx context.Context, req *agentpb.DownloadGeoRuleRequest) *agentpb.DownloadGeoRuleResponse {
	if req.FileName == "" || req.DownloadUrl == "" {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: "file_name and download_url are required"}
	}

	rulesDir := filepath.Join(a.dataDir, "rules")
	tmpDir := filepath.Join(a.dataDir, "rules_tmp")
	if err := os.MkdirAll(rulesDir, 0750); err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("create rules dir: %v", err)}
	}
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("create tmp dir: %v", err)}
	}

	// ⚠ 文件名直接 Join 会逃出 rules/（`../../../../etc/cron.d/x`）✗ —— 同文件里的
	// handleDeleteGeoRule 本来就有这个校验，这里是遗漏（安全审计 v5 H4）。
	if err := safepath.CheckName("geo rule file name", req.FileName); err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}
	filePath := filepath.Join(rulesDir, req.FileName)
	tmpPath := filepath.Join(tmpDir, req.FileName)

	// Hash comparison: if panel sent a file hash and the local file exists,
	// compute its hash and skip download if they match (saves bandwidth).
	if req.FileHash != "" {
		if existing, err := os.ReadFile(filePath); err == nil {
			h := sha256.Sum256(existing)
			if hex.EncodeToString(h[:]) == req.FileHash {
				log.Printf("[Agent] geo rule %s unchanged (hash match), skipping download", req.FileName)
				return &agentpb.DownloadGeoRuleResponse{
					Success:  true,
					FileSize: int64(len(existing)),
					FilePath: filePath,
				}
			}
		}
	}

	// Resolve relative download URL using the public HTTP endpoint from config
	// (CDN when configured). The panel sends relative paths like
	// "/api/v1/geo-rules/files/3/download-file" so it doesn't need to know its
	// own external URL — the Agent resolves it (same as UpdateAgent).
	downloadURL := req.DownloadUrl
	if strings.HasPrefix(downloadURL, "/") {
		downloadURL = a.panelHTTPBaseURL() + downloadURL
		log.Printf("[Agent] resolved relative geo rule download URL to %s", downloadURL)
	}

	// Apply GitHub acceleration domain if provided (only for GitHub URLs)
	if req.AccelDomain != "" {
		downloadURL = netutil.AccelerateGitHubURL(downloadURL, req.AccelDomain)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}

	// Use HTTP client with GitHub acceleration and/or proxy support
	client := netutil.GitHubHTTPClientWithProxy(120*time.Second, req.AccelDomain, req.ProxyUrl)
	resp, err := client.Do(httpReq)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("download failed: HTTP %d", resp.StatusCode)}
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}

	size, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}
	}

	log.Printf("[Agent] geo rule downloaded: %s (%d bytes) → %s", req.FileName, size, filePath)
	return &agentpb.DownloadGeoRuleResponse{
		Success:  true,
		FileSize: size,
		FilePath: filePath,
	}
}

// handleDeleteGeoRule deletes a geo rule file from the Agent's local data/rules/ directory.
// Called when the panel deletes a geo rule file and propagates the deletion to all nodes.
func (a *AgentApp) handleDeleteGeoRule(req *agentpb.DeleteGeoRuleRequest) *agentpb.DeleteGeoRuleResponse {
	if req.FileName == "" {
		return &agentpb.DeleteGeoRuleResponse{Success: false, Error: "file_name is required"}
	}

	// Prevent path traversal
	if strings.Contains(req.FileName, "..") || strings.Contains(req.FileName, "/") {
		return &agentpb.DeleteGeoRuleResponse{Success: false, Error: "invalid file name"}
	}

	filePath := filepath.Join(a.dataDir, "rules", req.FileName)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist — treat as success (idempotent)
			log.Printf("[Agent] geo rule file %s not found, already removed", req.FileName)
			return &agentpb.DeleteGeoRuleResponse{
				Success:  true,
				FilePath: filePath,
			}
		}
		return &agentpb.DeleteGeoRuleResponse{Success: false, Error: err.Error()}
	}

	log.Printf("[Agent] geo rule deleted: %s → %s", req.FileName, filePath)
	return &agentpb.DeleteGeoRuleResponse{
		Success:  true,
		FilePath: filePath,
	}
}
