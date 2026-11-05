package warp

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// DeleteResult 描述一次删除操作里本地与云端的执行结果。
type DeleteResult struct {
	RemoteAttempted bool   `json:"remote_attempted"`
	RemoteDeleted   bool   `json:"remote_deleted"`
	RemoteError     string `json:"remote_error,omitempty"`
}

var (
	// 抽成变量便于单测替换（httptest）。
	warpAPIBaseURL   = "https://api.cloudflareclient.com"
	warpDeleteClient = &http.Client{Timeout: 30 * time.Second}
)

// Delete 删除本地 WARP 记录；remote=true 时先尝试注销 Cloudflare 侧设备。
//
// 语义：本地删除优先。云端注销失败不会阻止本地删除（否则用户会被一个
// 已失效的设备记录卡住），失败原因通过 DeleteResult.RemoteError 返回。
func (s *Service) Delete(warpID int64, remote bool) (*DeleteResult, error) {
	cfg, err := s.repo.GetByID(nil, warpID)
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("warp config %d not found", warpID)
	}

	res := &DeleteResult{}
	if remote {
		res.RemoteAttempted = true
		switch {
		case cfg.DeviceID == "" || cfg.Token == "":
			res.RemoteError = "记录缺少 device_id/token，无法注销云端设备"
		default:
			if err := deleteWarpDevice(cfg); err != nil {
				res.RemoteError = err.Error()
			} else {
				res.RemoteDeleted = true
			}
		}
	}

	if err := s.repo.Delete(nil, warpID); err != nil {
		return nil, fmt.Errorf("delete warp config: %w", err)
	}
	return res, nil
}

// deleteWarpDevice 调 Cloudflare 的 DELETE /reg/{id} 注销设备。
//
// 两种注册形态使用的 API 版本不同（WireGuard 与 MASQUE 各自注册时的路径/版本头），
// 按记录的 TunnelType 选择，避免版本路径与 CF-Client-Version 不匹配被拒。
// 404 视为已删除（幂等），便于重复清理。
func deleteWarpDevice(cfg *entity.WarpConfig) error {
	base := warpAPIBaseURL + "/v0a2158"
	version := "a-6.10-2158"
	userAgent := "okhttp/3.12.1"
	if strings.EqualFold(cfg.TunnelType, masqueTunnel) {
		base = warpAPIBaseURL + "/v0a4471"
		version = masqueClientVersion
		userAgent = masqueUserAgent
	}

	url := base + "/reg/" + cfg.DeviceID
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	req.Header.Set("CF-Client-Version", version)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := warpDeleteClient.Do(req)
	if err != nil {
		return fmt.Errorf("注销云端设备失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return fmt.Errorf("注销云端设备失败: HTTP %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
