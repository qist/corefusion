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
	"time"

	agent "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
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
			err := updater.PerformUpdate(
				c.UpdateAgent.Version,
				c.UpdateAgent.DownloadUrl,
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
