package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	coreconfigSvc "corefusion/internal/service/coreconfig"
)

// clash_api 的 /connections 可能很大（几千条连接），限制读取上限避免打爆内存。
const maxSingboxConnectionsBody = 32 << 20

// singboxClashAPI 从运行时配置解析 clash_api 的监听地址与 secret。
//
// 为什么由 Agent 自己解析：clash_api 只监听节点回环（127.0.0.1:9090），
// 面板直连不到远端节点；而「实际生效的 listen/secret」只有节点上的
// runtime/singbox.jsonc 最准确（面板数据库里的配置可能被节点本地改写）。
func singboxClashAPI(dataDir string) (listen, secret string) {
	if strings.TrimSpace(dataDir) == "" {
		return "", ""
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "runtime", "singbox.jsonc"))
	if err != nil {
		return "", ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(coreconfigSvc.StripJSONC(raw), &cfg); err != nil {
		return "", ""
	}
	exp, _ := cfg["experimental"].(map[string]interface{})
	if exp == nil {
		return "", ""
	}
	clash, _ := exp["clash_api"].(map[string]interface{})
	if clash == nil {
		return "", ""
	}
	listen, _ = clash["listen"].(string)
	secret, _ = clash["secret"].(string)
	return strings.TrimSpace(listen), strings.TrimSpace(secret)
}

// fetchSingboxConnections 读取本机 sing-box clash_api 的 /connections 原始 JSON。
// 返回 (body, listen, error)；listen 用于排障（面板会在错误信息里展示）。
func (a *AgentApp) fetchSingboxConnections(ctx context.Context) (string, string, error) {
	if a.localAgent == nil {
		return "", "", fmt.Errorf("local agent not initialized")
	}
	listen, secret := singboxClashAPI(a.localAgent.GetDataDir())
	if listen == "" {
		return "", "", fmt.Errorf("该节点未启用 sing-box clash_api（experimental.clash_api.listen 为空），无法读取实时连接")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+listen+"/connections", nil)
	if err != nil {
		return "", listen, err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", listen, fmt.Errorf("请求 clash_api %s 失败：%w", listen, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", listen, fmt.Errorf("clash_api %s 返回 %d", listen, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSingboxConnectionsBody))
	if err != nil {
		return "", listen, fmt.Errorf("读取 clash_api 响应失败：%w", err)
	}
	return string(body), listen, nil
}
