package coredata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultSingboxClashAPIAddr 是 sing-box clash_api 的默认监听地址。
// 只监听回环：面板/Agent 在同一台机器上直接读 /connections 拿实时连接。
const DefaultSingboxClashAPIAddr = "127.0.0.1:9090"

// ClashConnection 是 sing-box clash_api `/connections` 返回的单条连接。
//
// 注意：sing-box 的 connections **不包含用户（订阅）字段**（v2ray_api 也没有
// 按用户的在线 IP 接口），所以这里只能提供「源 IP + 入站 + 目标 + 流量 + 起始
// 时间」，无法直接归户到订阅；订阅级在线用户仍以 xray 为准。
type ClashConnection struct {
	ID       string   `json:"id"`
	Upload   int64    `json:"upload"`
	Download int64    `json:"download"`
	Start    string   `json:"start"`
	Chains   []string `json:"chains"`
	Rule     string   `json:"rule"`
	Metadata struct {
		Network         string `json:"network"`
		Type            string `json:"type"` // 形如 "anytls/inbound-anytls-49146"
		SourceIP        string `json:"sourceIP"`
		DestinationIP   string `json:"destinationIP"`
		SourcePort      string `json:"sourcePort"`
		DestinationPort string `json:"destinationPort"`
		Host            string `json:"host"`
	} `json:"metadata"`
}

// ClashConnectionsResponse 是 `/connections` 的响应体。
type ClashConnectionsResponse struct {
	DownloadTotal int64             `json:"downloadTotal"`
	UploadTotal   int64             `json:"uploadTotal"`
	Connections   []ClashConnection `json:"connections"`
}

// ResolveClashAPIAddr 解析 clash_api 地址：环境变量 > 默认回环地址。
func ResolveClashAPIAddr() string {
	if v := strings.TrimSpace(os.Getenv("CF_SINGBOX_CLASH_API")); v != "" {
		return v
	}
	return DefaultSingboxClashAPIAddr
}

// FetchClashConnections 读取 sing-box 的实时连接列表。
func FetchClashConnections(ctx context.Context, addr string) (*ClashConnectionsResponse, error) {
	if strings.TrimSpace(addr) == "" {
		addr = ResolveClashAPIAddr()
	}
	url := "http://" + addr + "/connections"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect sing-box clash_api %s: %w", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sing-box clash_api %s returned %d", addr, resp.StatusCode)
	}
	var out ClashConnectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode clash_api response: %w", err)
	}
	return &out, nil
}
