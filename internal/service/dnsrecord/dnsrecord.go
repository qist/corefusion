// Package dnsrecord 负责「把某个域名的解析记录指向指定 IP / 主机名」，用于主备切换后
// 入口地址自动漂移（P2）。
//
// 抽象：每家 DNS 服务商实现同一个 Updater 接口（Upsert/Delete），Service 只按
// dns_channels 里的 provider 取对应适配器。新增服务商 = 注册一个 Factory。
//
// 注意：证书的 DNS-01 与这里的「解析写入」不是同一能力 —— lego 只暴露
// Present/CleanUp（写/删 TXT），不提供通用的记录查询/更新接口，因此其它服务商
// 需要在 dnsrecord 里单独实现（目前：cloudflare 已验证，alidns 已实现未实测）。
package dnsrecord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	secretcrypto "corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

// Service 用 dns_channels 里保存的凭据操作 DNS 记录。
type Service struct {
	channels *repository.DNSChannelRepository
	apiBase  string
	httpc    *http.Client
}

// NewService 创建 DNS 记录服务。
func NewService(channels *repository.DNSChannelRepository) *Service {
	return &Service{
		channels: channels,
		httpc:    &http.Client{Timeout: 15 * time.Second},
	}
}

// WithAPIBase 仅供单测指向 httptest（对 Cloudflare/支持自定义 base 的适配器生效）。
func (s *Service) WithAPIBase(base string) *Service {
	s.apiBase = strings.TrimRight(base, "/")
	return s
}

// UpsertA 把 fqdn 的 A 记录指向 target（IPv4）。proxied 仅 Cloudflare 生效（橙云）。
func (s *Service) UpsertA(ctx context.Context, channelID int64, fqdn, target string, proxied bool) error {
	fqdn = strings.TrimSpace(strings.TrimSuffix(fqdn, "."))
	target = strings.TrimSpace(target)
	if fqdn == "" {
		return fmt.Errorf("记录名为空")
	}
	ip := net.ParseIP(target)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("%q 不是合法 IPv4 地址（A 记录不支持 IPv6，请用 AAAA）", target)
	}
	return s.Upsert(ctx, channelID, Record{Type: "A", Name: fqdn, Content: target, TTL: 60, Proxied: proxied})
}

// Upsert 通用的建/改记录入口（A / AAAA / CNAME / TXT 由适配器决定支持范围）。
func (s *Service) Upsert(ctx context.Context, channelID int64, rec Record) error {
	up, err := s.updater(ctx, channelID)
	if err != nil {
		return err
	}
	rec.Name = strings.TrimSpace(strings.TrimSuffix(rec.Name, "."))
	rec.Type = strings.ToUpper(strings.TrimSpace(rec.Type))
	if rec.Name == "" || rec.Type == "" || strings.TrimSpace(rec.Content) == "" {
		return fmt.Errorf("记录名/类型/值不能为空")
	}
	return up.Upsert(ctx, rec)
}

// Delete 删除某域名的指定类型记录（幂等）。
func (s *Service) Delete(ctx context.Context, channelID int64, fqdn, typ string) error {
	up, err := s.updater(ctx, channelID)
	if err != nil {
		return err
	}
	return up.Delete(ctx, strings.TrimSpace(strings.TrimSuffix(fqdn, ".")), strings.ToUpper(strings.TrimSpace(typ)))
}

// updater 读取通道凭据并构造对应服务商的适配器。
func (s *Service) updater(ctx context.Context, channelID int64) (Updater, error) {
	if s == nil || s.channels == nil {
		return nil, fmt.Errorf("DNS 服务未初始化")
	}
	if channelID <= 0 {
		return nil, fmt.Errorf("未选择 DNS 通道")
	}
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("读取 DNS 通道 %d 失败：%w", channelID, err)
	}
	return s.updaterForChannel(ch)
}

func (s *Service) updaterForChannel(ch *entity.DNSChannel) (Updater, error) {
	if ch == nil {
		return nil, fmt.Errorf("DNS 通道为空")
	}
	if ch.Status != "" && ch.Status != "active" {
		return nil, fmt.Errorf("DNS 通道 %s 状态是 %s，不可用", ch.Name, ch.Status)
	}
	vars, err := decodeChannelEnv(ch.EnvVars)
	if err != nil {
		return nil, err
	}
	up, err := newUpdater(ch.Provider, vars, ProviderConfig{HTTPClient: s.httpc, APIBase: s.apiBase})
	if err != nil {
		return nil, fmt.Errorf("通道 %s：%w", ch.Name, err)
	}
	return up, nil
}

func decodeChannelEnv(encoded string) (map[string]string, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, fmt.Errorf("DNS 通道没有配置凭据")
	}
	plain, err := secretcrypto.DecryptSecret(encoded)
	if err != nil {
		return nil, fmt.Errorf("解密 DNS 凭据失败：%w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(plain)
	if err != nil {
		return nil, fmt.Errorf("解码 DNS 凭据失败：%w", err)
	}
	var vars map[string]string
	if err := json.Unmarshal(raw, &vars); err != nil {
		return nil, fmt.Errorf("解析 DNS 凭据失败：%w", err)
	}
	return vars, nil
}

func firstVar(vars map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(vars[k]); v != "" {
			return v
		}
	}
	return ""
}

// ParseUint 是给调用方从字符串配置解析通道 ID 的小工具。
func ParseUint(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}
