package dnsrecord

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	alidns "github.com/go-acme/alidns-20150109/v5/client"
)

// alidnsUpdater 是阿里云 DNS 的解析写入适配器。
//
// 凭据键与证书 DNS-01 通道一致：ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY，
// 可选 ALICLOUD_REGION_ID / ALICLOUD_ENDPOINT。
// 状态：已实现，但尚未用真实账号验证（前端会标注「未实测」）。
type alidnsUpdater struct {
	client *alidns.Client
}

func newAliDNSUpdater(vars map[string]string, _ ProviderConfig) (Updater, error) {
	ak := firstVar(vars, "ALICLOUD_ACCESS_KEY", "ALIYUN_ACCESS_KEY_ID", "ALI_ACCESS_KEY_ID")
	sk := firstVar(vars, "ALICLOUD_SECRET_KEY", "ALIYUN_ACCESS_KEY_SECRET", "ALI_SECRET_KEY")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("缺少阿里云凭据（ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY）")
	}
	cfg := new(openapi.Config).SetAccessKeyId(ak).SetAccessKeySecret(sk)
	if region := firstVar(vars, "ALICLOUD_REGION_ID"); region != "" {
		cfg.SetRegionId(region)
	}
	if endpoint := firstVar(vars, "ALICLOUD_ENDPOINT"); endpoint != "" {
		cfg.SetEndpoint(endpoint)
	}
	client, err := alidns.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化阿里云 DNS 客户端失败：%w", err)
	}
	return &alidnsUpdater{client: client}, nil
}

func (u *alidnsUpdater) Provider() string { return "alidns" }

func (u *alidnsUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, rec.Name)
	if err != nil {
		return err
	}
	rr := recordRR(rec.Name, zone)
	if rr == "" {
		return fmt.Errorf("%s 不是 %s 的子域名", rec.Name, zone)
	}
	records, err := u.listRecords(ctx, zone, rr, typ)
	if err != nil {
		return err
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 60
	}
	if len(records) > 0 {
		for _, r := range records[1:] {
			if err := u.deleteByID(ctx, derefStr(r.GetRecordId())); err != nil {
				return fmt.Errorf("清理重复 %s 记录失败：%w", typ, err)
			}
		}
		req := new(alidns.UpdateDomainRecordRequest).
			SetRecordId(derefStr(records[0].GetRecordId())).
			SetRR(rr).
			SetType(typ).
			SetValue(rec.Content).
			SetTTL(int64(ttl))
		if _, err := alidns.UpdateDomainRecordWithContext(ctx, u.client, req, &dara.RuntimeOptions{}); err != nil {
			return fmt.Errorf("更新 %s 记录 %s → %s 失败：%w", typ, rec.Name, rec.Content, err)
		}
		return nil
	}
	req := new(alidns.AddDomainRecordRequest).
		SetDomainName(zone).
		SetRR(rr).
		SetType(typ).
		SetValue(rec.Content).
		SetTTL(int64(ttl))
	if _, err := alidns.AddDomainRecordWithContext(ctx, u.client, req, &dara.RuntimeOptions{}); err != nil {
		return fmt.Errorf("创建 %s 记录 %s → %s 失败：%w", typ, rec.Name, rec.Content, err)
	}
	return nil
}

func (u *alidnsUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zone, err := u.findZone(ctx, name)
	if err != nil {
		return err
	}
	rr := recordRR(name, zone)
	if rr == "" {
		return nil
	}
	records, err := u.listRecords(ctx, zone, rr, typ)
	if err != nil {
		return err
	}
	for _, r := range records {
		if err := u.deleteByID(ctx, derefStr(r.GetRecordId())); err != nil {
			return fmt.Errorf("删除 %s 记录 %s 失败：%w", typ, name, err)
		}
	}
	return nil
}

func (u *alidnsUpdater) deleteByID(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	req := new(alidns.DeleteDomainRecordRequest).SetRecordId(id)
	_, err := alidns.DeleteDomainRecordWithContext(ctx, u.client, req, &dara.RuntimeOptions{})
	return err
}

// findZone 列出账号下所有域名，取 fqdn 的**最长**后缀匹配（支持子 zone）。
func (u *alidnsUpdater) findZone(ctx context.Context, fqdn string) (string, error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	req := new(alidns.DescribeDomainsRequest).SetPageSize(100)
	best := ""
	total := int64(-1)
	for page := int64(1); page <= 100; page++ {
		req.SetPageNumber(page)
		resp, err := alidns.DescribeDomainsWithContext(ctx, u.client, req, &dara.RuntimeOptions{})
		if err != nil {
			return "", fmt.Errorf("查询阿里云域名列表失败：%w", err)
		}
		if resp == nil || resp.Body == nil || resp.Body.Domains == nil {
			break
		}
		for _, d := range resp.Body.Domains.Domain {
			name := strings.ToLower(derefStr(d.GetDomainName()))
			if name == "" {
				continue
			}
			if (fqdn == name || strings.HasSuffix(fqdn, "."+name)) && len(name) > len(best) {
				best = name
			}
		}
		if resp.Body.TotalCount != nil {
			total = *resp.Body.TotalCount
		}
		if total >= 0 && page*100 >= total {
			break
		}
		if len(resp.Body.Domains.Domain) == 0 {
			break
		}
	}
	if best == "" {
		return "", fmt.Errorf("凭据看不到 %s 所属的阿里云域名（未授权或不在该账号）", fqdn)
	}
	return best, nil
}

func (u *alidnsUpdater) listRecords(ctx context.Context, zone, rr, typ string) ([]*alidns.DescribeDomainRecordsResponseBodyDomainRecordsRecord, error) {
	req := new(alidns.DescribeDomainRecordsRequest).
		SetDomainName(zone).
		SetRRKeyWord(rr).
		SetType(typ).
		SetPageSize(500)
	resp, err := alidns.DescribeDomainRecordsWithContext(ctx, u.client, req, &dara.RuntimeOptions{})
	if err != nil {
		return nil, fmt.Errorf("查询 %s 记录失败：%w", typ, err)
	}
	var out []*alidns.DescribeDomainRecordsResponseBodyDomainRecordsRecord
	if resp == nil || resp.Body == nil || resp.Body.DomainRecords == nil {
		return out, nil
	}
	for _, r := range resp.Body.DomainRecords.Record {
		if strings.EqualFold(derefStr(r.GetRR()), rr) && strings.EqualFold(derefStr(r.GetType()), typ) {
			out = append(out, r)
		}
	}
	return out, nil
}

// recordRR 把 FQDN 转成阿里云要求的 RR（主机记录）：entry.a.com @ a.com → entry。
func recordRR(fqdn, zone string) string {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	zone = strings.ToLower(strings.TrimSuffix(zone, "."))
	if fqdn == zone {
		return "@"
	}
	if strings.HasSuffix(fqdn, "."+zone) {
		return strings.TrimSuffix(fqdn, "."+zone)
	}
	return ""
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
