package main

import (
	"context"
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
	logManageSvc "corefusion/internal/service/logmanage"
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
		if err := a.localAgent.SyncConfig(ctx, req.CoreType, req.ConfigContent, req.ConfigVersion); err != nil {
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
		if err := a.localAgent.SyncProxyConfig(ctx, req.ProxyType, req.ConfigContent); err != nil {
			log.Printf("[Agent] proxy config sync failed: %s: %v", req.ProxyType, err)
		} else {
			log.Printf("[Agent] proxy config synced: %s (%d certs)", req.ProxyType, len(req.Certificates))
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

	case *agentpb.PanelCommand_DistributeCert:
		a.handleDistributeCert(c.DistributeCert)

	case *agentpb.PanelCommand_GetStatus:
		a.sendStatusReport(stream)

	case *agentpb.PanelCommand_UpdateAgent:
		log.Printf("[Agent] UpdateAgent command received: version=%s", c.UpdateAgent.Version)
		go func() {
			updater := agent.NewSelfUpdater(a.dataDir, a.settingRepo)
			// Resolve relative download URL using the panel address from config
			downloadURL := c.UpdateAgent.DownloadUrl
			if strings.HasPrefix(downloadURL, "/") {
				scheme := "http"
				if a.cfg.Panel.TLS {
					scheme = "https"
				}
				downloadURL = fmt.Sprintf("%s://%s:%d%s", scheme, a.cfg.Panel.Host, a.cfg.Panel.Port, downloadURL)
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
		cpu, mem := getSystemMetrics()
		resp := &agentpb.GetMetricsResponse{
			Points: []*agentpb.MetricPoint{{
				Timestamp:     time.Now().Unix(),
				CpuUsage:      cpu,
				MemUsageBytes: mem,
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
		// TODO: implement SpeedTest logic
		sendSyncResult("speed_test", &agentpb.SpeedTestResponse{
			Success: false, Error: "not implemented",
		}, nil)

	case *agentpb.PanelCommand_DownloadGeoRule:
		req := c.DownloadGeoRule
		resp := a.handleDownloadGeoRule(ctx, req)
		sendSyncResult("download_geo_rule", resp, nil)

	case *agentpb.PanelCommand_ListInterfaces:
		names, err := a.localAgent.ListInterfaces()
		sendSyncResult("list_interfaces", &agentpb.ListInterfacesResponse{Interfaces: names}, err)

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
					Nginx struct{ Mode string `json:"mode"` } `json:"nginx"`
					Caddy struct{ Mode string `json:"mode"` } `json:"caddy"`
				}
				if err := json.Unmarshal([]byte(req.ProxyLogConfig), &proxyCfg); err == nil {
					if err := a.logMgr.StartWriter("nginx", proxyCfg.Nginx.Mode); err != nil {
						log.Printf("[Agent] failed to start nginx writer: %v", err)
					}
					if err := a.logMgr.StartWriter("caddy", proxyCfg.Caddy.Mode); err != nil {
						log.Printf("[Agent] failed to start caddy writer: %v", err)
					}
				}
			}
			// Parse core config and start/stop writers for xray/singbox/hysteria2/tuic
			if req.CoreLogConfig != "" {
				var coreCfg struct {
					Xray      struct{ Mode string `json:"mode"` } `json:"xray"`
					Singbox   struct{ Mode string `json:"mode"` } `json:"singbox"`
					Hysteria2 struct{ Mode string `json:"mode"` } `json:"hysteria2"`
					Tuic      struct{ Mode string `json:"mode"` } `json:"tuic"`
				}
				if err := json.Unmarshal([]byte(req.CoreLogConfig), &coreCfg); err == nil {
					if err := a.logMgr.StartWriter("xray", coreCfg.Xray.Mode); err != nil {
						log.Printf("[Agent] failed to start xray writer: %v", err)
					}
					if err := a.logMgr.StartWriter("singbox", coreCfg.Singbox.Mode); err != nil {
						log.Printf("[Agent] failed to start singbox writer: %v", err)
					}
					if err := a.logMgr.StartWriter("hysteria2", coreCfg.Hysteria2.Mode); err != nil {
						log.Printf("[Agent] failed to start hysteria2 writer: %v", err)
					}
					if err := a.logMgr.StartWriter("tuic", coreCfg.Tuic.Mode); err != nil {
						log.Printf("[Agent] failed to start tuic writer: %v", err)
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

	default:
		log.Printf("[Agent] unknown command: %T", cmd.Command)
	}
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
	targetPath := cmd.TargetPath
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(a.dataDir, targetPath)
	}
	dir := filepath.Dir(targetPath)
	os.MkdirAll(dir, 0750)

	// Write cert.pem
	if err := os.WriteFile(targetPath, cmd.CertPem, 0640); err != nil {
		log.Printf("[Agent] distribute cert write error: %v", err)
		return
	}
	// Write key alongside (same dir, key.pem)
	keyPath := filepath.Join(dir, "key.pem")
	os.WriteFile(keyPath, cmd.KeyPem, 0600)
	if len(cmd.FullchainPem) > 0 {
		fullchainPath := filepath.Join(dir, "fullchain.pem")
		os.WriteFile(fullchainPath, cmd.FullchainPem, 0640)
	}

	log.Printf("[Agent] certificate %s distributed to %s", cmd.CertName, dir)

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

	filePath := filepath.Join(rulesDir, req.FileName)
	tmpPath := filepath.Join(tmpDir, req.FileName)

	// Apply GitHub acceleration domain if provided
	downloadURL := req.DownloadUrl
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
