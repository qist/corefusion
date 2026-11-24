package dnsrecord

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	hwauthbasic "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	hwconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	hwdns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	hwmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	hwregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

// huaweiUpdater 是华为云 DNS 的解析写入适配器。
//
// 凭据：HWC_ACCESS_KEY / HWC_SECRET_KEY / HWC_REGION（与证书通道一致），
// 兼容 lego 的 HUAWEICLOUD_ACCESS_KEY_ID / HUAWEICLOUD_SECRET_ACCESS_KEY /
// HUAWEICLOUD_REGION。
// 状态：已实现，未实测。
type huaweiUpdater struct {
	client *hwdns.DnsClient
}

func newHuaweiUpdater(vars map[string]string, _ ProviderConfig) (Updater, error) {
	ak := firstVar(vars, "HWC_ACCESS_KEY", "HUAWEICLOUD_ACCESS_KEY_ID", "HUAWEI_ACCESS_KEY")
	sk := firstVar(vars, "HWC_SECRET_KEY", "HUAWEICLOUD_SECRET_ACCESS_KEY", "HUAWEI_SECRET_KEY")
	regionID := firstVar(vars, "HWC_REGION", "HUAWEICLOUD_REGION", "HUAWEI_REGION")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("缺少华为云凭据（HWC_ACCESS_KEY / HWC_SECRET_KEY）")
	}
	if regionID == "" {
		return nil, fmt.Errorf("缺少华为云区域（HWC_REGION，如 cn-north-4）")
	}
	auth, err := hwauthbasic.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("初始化华为云凭据失败：%w", err)
	}
	region, err := hwregion.SafeValueOf(regionID)
	if err != nil {
		return nil, fmt.Errorf("不支持的华为云区域 %q：%w", regionID, err)
	}
	hc, err := hwdns.DnsClientBuilder().
		WithHttpConfig(hwconfig.DefaultHttpConfig()).
		WithRegion(region).
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("初始化华为云 DNS 客户端失败：%w", err)
	}
	return &huaweiUpdater{client: hwdns.NewDnsClient(hc)}, nil
}

func (u *huaweiUpdater) Provider() string { return "huawei" }

func (u *huaweiUpdater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zoneID, zoneName, err := u.findZone(rec.Name)
	if err != nil {
		return err
	}
	_ = ctx
	fqdn := fqdnDot(rec.Name)
	ttl := int32(rec.TTL)
	if ttl <= 0 {
		ttl = 60
	}
	value := huaweiRecordValue(typ, rec.Content)
	existing, err := u.getRecordset(zoneID, fqdn, typ)
	if err != nil {
		return err
	}
	_ = zoneName
	if existing == nil {
		_, err = u.client.CreateRecordSet(&hwmodel.CreateRecordSetRequest{
			ZoneId: zoneID,
			Body: &hwmodel.CreateRecordSetRequestBody{
				Name:    fqdn,
				Type:    typ,
				Ttl:     &ttl,
				Records: []string{value},
			},
		})
		if err != nil {
			return fmt.Errorf("创建华为云 %s 记录 %s 失败：%w", typ, rec.Name, err)
		}
		return nil
	}
	desc := existing.Description
	_, err = u.client.UpdateRecordSet(&hwmodel.UpdateRecordSetRequest{
		ZoneId:      zoneID,
		RecordsetId: derefStr(existing.Id),
		Body: &hwmodel.UpdateRecordSetReq{
			Name:        existing.Name,
			Description: desc,
			Type:        existing.Type,
			Ttl:         &ttl,
			Records:     &[]string{value},
		},
	})
	if err != nil {
		return fmt.Errorf("更新华为云 %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *huaweiUpdater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zoneID, _, err := u.findZone(name)
	if err != nil {
		return err
	}
	_ = ctx
	existing, err := u.getRecordset(zoneID, fqdnDot(name), typ)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	_, err = u.client.DeleteRecordSet(&hwmodel.DeleteRecordSetRequest{
		ZoneId:      zoneID,
		RecordsetId: derefStr(existing.Id),
	})
	if err != nil {
		return fmt.Errorf("删除华为云 %s 记录 %s 失败：%w", typ, name, err)
	}
	return nil
}

// findZone 列出所有公网 zone，取 fqdn 的最长后缀匹配。
func (u *huaweiUpdater) findZone(fqdn string) (zoneID, zoneName string, err error) {
	want := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	limit := int32(500)
	offset := int32(0)
	for page := 0; page < 20; page++ {
		resp, lerr := u.client.ListPublicZones(&hwmodel.ListPublicZonesRequest{Limit: &limit, Offset: &offset})
		if lerr != nil {
			return "", "", fmt.Errorf("查询华为云 zone 失败：%w", lerr)
		}
		if resp == nil || resp.Zones == nil {
			break
		}
		count := 0
		for _, z := range *resp.Zones {
			count++
			name := strings.ToLower(strings.TrimSuffix(derefStr(z.Name), "."))
			if name == "" {
				continue
			}
			if (want == name || strings.HasSuffix(want, "."+name)) && len(name) > len(zoneName) {
				zoneID, zoneName = derefStr(z.Id), name
			}
		}
		if count < int(limit) {
			break
		}
		offset += limit
	}
	if zoneID == "" {
		return "", "", fmt.Errorf("凭据看不到 %s 所属的华为云 zone", want)
	}
	return zoneID, zoneName, nil
}

func (u *huaweiUpdater) getRecordset(zoneID, fqdn, typ string) (*hwmodel.ListRecordSets, error) {
	resp, err := u.client.ListRecordSetsByZone(&hwmodel.ListRecordSetsByZoneRequest{
		ZoneId: zoneID,
		Name:   &fqdn,
	})
	if err != nil {
		return nil, fmt.Errorf("查询华为云 %s 记录失败：%w", typ, err)
	}
	if resp == nil || resp.Recordsets == nil {
		return nil, nil
	}
	for i := range *resp.Recordsets {
		rs := (*resp.Recordsets)[i]
		if strings.EqualFold(derefStr(rs.Type), typ) && strings.EqualFold(derefStr(rs.Name), fqdn) {
			return &rs, nil
		}
	}
	return nil, nil
}

func huaweiRecordValue(typ, content string) string {
	if typ == "TXT" && !strings.HasPrefix(content, `"`) {
		return strconv.Quote(content)
	}
	return content
}
