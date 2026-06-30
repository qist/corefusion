package agent

import (
	"context"

	proxySvc "corefusion/internal/service/proxy"
)

// ProxyManagerAdapter 包装 proxy.Manager 使其实现 ProxyManager 接口。
// proxy.Manager 的方法使用 ProxyType 类型参数（type ProxyType string），
// 而 ProxyManager 接口使用 string，需要适配器做类型转换。
type ProxyManagerAdapter struct {
	mgr *proxySvc.Manager
}

// NewProxyManagerAdapter 创建适配器。
func NewProxyManagerAdapter(mgr *proxySvc.Manager) *ProxyManagerAdapter {
	return &ProxyManagerAdapter{mgr: mgr}
}

func (a *ProxyManagerAdapter) StartProxy(ctx context.Context, proxyType string) error {
	return a.mgr.StartProxy(ctx, proxySvc.ProxyType(proxyType))
}

func (a *ProxyManagerAdapter) StopProxy(ctx context.Context, proxyType string) error {
	return a.mgr.StopProxy(ctx, proxySvc.ProxyType(proxyType))
}

func (a *ProxyManagerAdapter) RestartProxy(ctx context.Context, proxyType string) error {
	return a.mgr.RestartProxy(ctx, proxySvc.ProxyType(proxyType))
}

func (a *ProxyManagerAdapter) ReloadProxy(ctx context.Context, proxyType string) error {
	return a.mgr.ReloadProxy(ctx, proxySvc.ProxyType(proxyType))
}

func (a *ProxyManagerAdapter) WriteConfigFile(proxyType string, config string) error {
	return a.mgr.WriteConfigFile(proxySvc.ProxyType(proxyType), config)
}

func (a *ProxyManagerAdapter) DetectProxy(proxyType string) ProxyInfo {
	info := a.mgr.DetectProxy(proxySvc.ProxyType(proxyType))
	return ProxyInfo{
		Type:      string(info.Type),
		Name:      info.Name,
		Installed: info.Installed,
		Version:   info.Version,
		Path:      info.Path,
		Status:    info.Status,
		Managed:   info.Managed,
	}
}
