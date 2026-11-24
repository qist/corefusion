package dnsrecord

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

// googleUpdater 是 Google Cloud DNS 的解析写入适配器。
//
// 凭据：GCE_PROJECT（项目 ID）+ GCE_SERVICE_ACCOUNT_FILE（服务账号 JSON 路径，
// 服务账号需要 roles/dns.admin）。面板需要能读到该文件（远端节点不参与改解析）。
// 状态：已实现，未实测。
type googleUpdater struct {
	svc     *dns.Service
	project string
}

func newGoogleUpdater(vars map[string]string, _ ProviderConfig) (Updater, error) {
	project := firstVar(vars, "GCE_PROJECT", "GOOGLE_PROJECT", "GOOGLE_CLOUD_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("缺少 GCE_PROJECT（Google Cloud 项目 ID）")
	}
	saFile := firstVar(vars, "GCE_SERVICE_ACCOUNT_FILE", "GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_SA_FILE")
	opts := []option.ClientOption{option.WithScopes(dns.CloudPlatformScope)}
	if saFile != "" {
		if _, err := os.Stat(saFile); err != nil {
			return nil, fmt.Errorf("服务账号文件不可读：%s（%v）", saFile, err)
		}
		opts = append(opts, option.WithCredentialsFile(saFile))
	}
	svc, err := dns.NewService(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("初始化 Google Cloud DNS 客户端失败：%w", err)
	}
	return &googleUpdater{svc: svc, project: project}, nil
}

func (u *googleUpdater) Provider() string { return "google" }

func (u *googleUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, rec.Name)
	if err != nil {
		return err
	}
	current, err := u.getRRSet(ctx, zone, rec.Name, typ)
	if err != nil {
		return err
	}
	ttl := int64(rec.TTL)
	if ttl <= 0 {
		ttl = 60
	}
	want := &dns.ResourceRecordSet{
		Name:    fqdnDot(rec.Name),
		Type:    typ,
		Ttl:     ttl,
		Rrdatas: []string{googleRecordValue(typ, rec.Content)},
	}
	change := &dns.Change{Additions: []*dns.ResourceRecordSet{want}}
	if current != nil {
		change.Deletions = []*dns.ResourceRecordSet{current}
	}
	if _, err := u.svc.Changes.Create(u.project, zone, change).Context(ctx).Do(); err != nil {
		return fmt.Errorf("写入 Google Cloud DNS %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *googleUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, name)
	if err != nil {
		return err
	}
	current, err := u.getRRSet(ctx, zone, name, typ)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	change := &dns.Change{Deletions: []*dns.ResourceRecordSet{current}}
	if _, err := u.svc.Changes.Create(u.project, zone, change).Context(ctx).Do(); err != nil {
		return fmt.Errorf("删除 Google Cloud DNS %s 记录 %s 失败：%w", typ, name, err)
	}
	return nil
}

// findZone 返回 ManagedZone 的名称（不是 DNS 域名），按 DnsName 最长后缀匹配。
func (u *googleUpdater) findZone(ctx context.Context, fqdn string) (string, error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	best, bestLen := "", 0
	pageToken := ""
	for page := 0; page < 20; page++ {
		call := u.svc.ManagedZones.List(u.project).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		out, err := call.Do()
		if err != nil {
			return "", fmt.Errorf("查询 Google Cloud DNS 区域失败：%w", err)
		}
		for _, z := range out.ManagedZones {
			name := strings.ToLower(strings.TrimSuffix(z.DnsName, "."))
			if name == "" {
				continue
			}
			if (fqdn == name || strings.HasSuffix(fqdn, "."+name)) && len(name) > bestLen {
				best, bestLen = z.Name, len(name)
			}
		}
		if out.NextPageToken == "" {
			break
		}
		pageToken = out.NextPageToken
	}
	if best == "" {
		return "", fmt.Errorf("凭据看不到 %s 所属的 Google Cloud DNS 区域", fqdn)
	}
	return best, nil
}

func (u *googleUpdater) getRRSet(ctx context.Context, zone, name, typ string) (*dns.ResourceRecordSet, error) {
	out, err := u.svc.ResourceRecordSets.List(u.project, zone).
		Name(fqdnDot(name)).Type(typ).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("读取 Google Cloud DNS 记录失败：%w", err)
	}
	for _, rr := range out.Rrsets {
		if strings.EqualFold(rr.Name, fqdnDot(name)) && strings.EqualFold(rr.Type, typ) {
			return rr, nil
		}
	}
	return nil, nil
}

func googleRecordValue(typ, content string) string {
	if typ == "TXT" && !strings.HasPrefix(content, `"`) {
		return `"` + content + `"`
	}
	return content
}
