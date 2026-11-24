package dnsrecord

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Record 是一条 DNS 解析记录。
type Record struct {
	Type    string // A / AAAA / CNAME / TXT
	Name    string // 完整域名（FQDN）
	Content string
	TTL     int
	// Proxied 仅 Cloudflare 有意义（橙云）；其它服务商忽略。
	Proxied bool
}

// Updater 是所有 DNS 服务商适配器必须实现的统一接口。
//
// 语义要求：
//   - Upsert：同名同类型只保留一条（历史多条会收敛），不存在则创建；
//   - Delete：删除同名同类型的所有记录，不存在视为成功（幂等）。
type Updater interface {
	Provider() string
	Upsert(ctx context.Context, rec Record) error
	Delete(ctx context.Context, name, typ string) error
}

// ProviderConfig 是服务无关的运行参数（单测里用来指向 httptest）。
type ProviderConfig struct {
	HTTPClient *http.Client
	APIBase    string
}

// Factory 用通道凭据构造适配器。
type Factory func(vars map[string]string, cfg ProviderConfig) (Updater, error)

// ErrProviderNotSupported 表示该服务商还没有解析写入实现（证书能签发 ≠ 这里能用：
// lego 只暴露 DNS-01 的 Present/CleanUp，不提供通用的记录 CRUD 接口）。
var ErrProviderNotSupported = errors.New("该 DNS 服务商的解析写入尚未实现")

// updaterFactories 已实现的服务商。新增服务商只需在这里注册一个 Factory。
var updaterFactories = map[string]Factory{
	"cloudflare": newCloudflareUpdater,
	"alidns":     newAliDNSUpdater,
	"dnspod":     newDNSPodUpdater,
	"godaddy":    newGoDaddyUpdater,
	"namecheap":  newNamecheapUpdater,
	"route53":    newRoute53Updater,
	"google":     newGoogleUpdater,
	"huawei":     newHuaweiUpdater,
}

// verifiedProviders 是做过真机验证（建/改/删 + 权威 NS 生效）的服务商。
// 未验证的实现仍然可用，但前端会标注「未实测」。
var verifiedProviders = map[string]bool{
	"cloudflare": true,
}

// ProviderCapability 描述某服务商的自动解析能力（前端据此决定能否选择）。
type ProviderCapability struct {
	Provider    string `json:"provider"`
	Implemented bool   `json:"implemented"`
	Verified    bool   `json:"verified"`
	Note        string `json:"note,omitempty"`
}

// Capabilities 返回服务商能力表。known 是「证书 DNS-01 已支持的服务商」列表
// （由调用方传入，避免本包依赖 certificate 包）；未实现的服务商也会列出，
// 以便前端解释为什么不能选。
func Capabilities(known []string) []ProviderCapability {
	seen := make(map[string]bool, len(known)+len(updaterFactories))
	out := make([]ProviderCapability, 0, len(known)+len(updaterFactories))
	add := func(p string) {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		_, impl := updaterFactories[p]
		cap := ProviderCapability{Provider: p, Implemented: impl, Verified: impl && verifiedProviders[p]}
		switch {
		case !impl:
			cap.Note = "证书签发可用，但自动改解析尚未实现；请在 DNS 服务商处手工维护"
		case !cap.Verified:
			cap.Note = "已实现但未实测，建议先用测试记录验证"
		}
		out = append(out, cap)
	}
	for _, p := range known {
		add(p)
	}
	for p := range updaterFactories {
		add(p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

// newUpdater 按服务商构造适配器。
func newUpdater(provider string, vars map[string]string, cfg ProviderConfig) (Updater, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	f, ok := updaterFactories[p]
	if !ok {
		impl := make([]string, 0, len(updaterFactories))
		for name := range updaterFactories {
			impl = append(impl, name)
		}
		sort.Strings(impl)
		return nil, fmt.Errorf("%s：%w（当前已实现：%s）", provider, ErrProviderNotSupported, strings.Join(impl, "、"))
	}
	up, err := f(vars, cfg)
	if err != nil {
		return nil, err
	}
	if up == nil {
		return nil, fmt.Errorf("%s：适配器初始化失败", provider)
	}
	return up, nil
}
