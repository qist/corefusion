package dnsrecord

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// route53Updater 是 AWS Route53 的解析写入适配器。
//
// 凭据：AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY（+ 可选 AWS_REGION，Route53 全局但
// SDK 需要一个 region 占位，默认 us-east-1）。
// 状态：已实现，未实测。
type route53Updater struct {
	client *route53.Client
}

func newRoute53Updater(vars map[string]string, _ ProviderConfig) (Updater, error) {
	ak := firstVar(vars, "AWS_ACCESS_KEY_ID", "AWS_KEY")
	sk := firstVar(vars, "AWS_SECRET_ACCESS_KEY", "AWS_SECRET")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("缺少 AWS 凭据（AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY）")
	}
	region := firstVar(vars, "AWS_REGION", "AWS_DEFAULT_REGION")
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(ak, sk, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化 AWS 配置失败：%w", err)
	}
	return &route53Updater{client: route53.NewFromConfig(cfg)}, nil
}

func (u *route53Updater) Provider() string { return "route53" }

func (u *route53Updater) Upsert(ctx context.Context, rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "A"
	}
	zoneID, err := u.findZone(ctx, rec.Name)
	if err != nil {
		return err
	}
	ttl := int64(rec.TTL)
	if ttl <= 0 {
		ttl = 60
	}
	change := route53types.Change{
		Action: route53types.ChangeActionUpsert,
		ResourceRecordSet: &route53types.ResourceRecordSet{
			Name:            aws.String(fqdnDot(rec.Name)),
			Type:            route53types.RRType(typ),
			TTL:             aws.Int64(ttl),
			ResourceRecords: []route53types.ResourceRecord{{Value: aws.String(route53Value(typ, rec.Content))}},
		},
	}
	_, err = u.client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch:  &route53types.ChangeBatch{Changes: []route53types.Change{change}},
	})
	if err != nil {
		return fmt.Errorf("写入 Route53 %s 记录 %s 失败：%w", typ, rec.Name, err)
	}
	return nil
}

func (u *route53Updater) Delete(ctx context.Context, name, typ string) error {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		typ = "A"
	}
	zoneID, err := u.findZone(ctx, name)
	if err != nil {
		return err
	}
	current, err := u.getRecordSet(ctx, zoneID, name, typ)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	_, err = u.client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &route53types.ChangeBatch{Changes: []route53types.Change{{
			Action:            route53types.ChangeActionDelete,
			ResourceRecordSet: current,
		}}},
	})
	if err != nil {
		return fmt.Errorf("删除 Route53 %s 记录 %s 失败：%w", typ, name, err)
	}
	return nil
}

// findZone 列出 Hosted Zones，取 fqdn 的最长后缀匹配。
func (u *route53Updater) findZone(ctx context.Context, fqdn string) (string, error) {
	fqdn = strings.ToLower(strings.TrimSuffix(fqdn, "."))
	bestID, bestLen := "", 0
	var marker *string
	for page := 0; page < 20; page++ {
		out, err := u.client.ListHostedZones(ctx, &route53.ListHostedZonesInput{Marker: marker, MaxItems: aws.Int32(100)})
		if err != nil {
			return "", fmt.Errorf("查询 Route53 Hosted Zone 失败：%w", err)
		}
		for _, z := range out.HostedZones {
			name := strings.ToLower(strings.TrimSuffix(aws.ToString(z.Name), "."))
			if name == "" {
				continue
			}
			if (fqdn == name || strings.HasSuffix(fqdn, "."+name)) && len(name) > bestLen {
				bestID, bestLen = aws.ToString(z.Id), len(name)
			}
		}
		if out.IsTruncated && out.NextMarker != nil {
			marker = out.NextMarker
			continue
		}
		break
	}
	if bestID == "" {
		return "", fmt.Errorf("凭据看不到 %s 所属的 Route53 Hosted Zone", fqdn)
	}
	return bestID, nil
}

func (u *route53Updater) getRecordSet(ctx context.Context, zoneID, name, typ string) (*route53types.ResourceRecordSet, error) {
	want := fqdnDot(name)
	out, err := u.client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(zoneID),
		StartRecordName: aws.String(want),
		StartRecordType: route53types.RRType(typ),
		MaxItems:        aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("读取 Route53 记录失败：%w", err)
	}
	if len(out.ResourceRecordSets) == 0 {
		return nil, nil
	}
	rr := out.ResourceRecordSets[0]
	if !strings.EqualFold(aws.ToString(rr.Name), want) || string(rr.Type) != typ {
		return nil, nil
	}
	return &rr, nil
}

func fqdnDot(name string) string {
	n := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	return n + "."
}

// route53Value Route53 的 TXT 记录值必须带引号。
func route53Value(typ, content string) string {
	if typ == "TXT" && !strings.HasPrefix(content, `"`) {
		return `"` + content + `"`
	}
	return content
}
