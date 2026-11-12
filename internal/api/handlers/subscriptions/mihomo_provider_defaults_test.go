package subscriptions

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"corefusion/internal/model/entity"
)

func providerTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/sub/token", nil)
	return c
}

func findProviderDef(defs []mihomoProviderDefinition, key string) map[string]interface{} {
	for _, d := range defs {
		if d.Key == key {
			return d.Fields
		}
	}
	return nil
}

// internal_defaults 要能控制自动生成的内部 provider：
// path 支持 {key} 占位，多 provider 不会互相覆盖；health-check 可换成自定义探测地址。
// subscription-nodes 不再作为可见 provider 输出（节点组已覆盖全部节点）。
func TestProviderDefaultsApplyToInternalProviders(t *testing.T) {
	tmpl := &entity.PlanTemplate{
		NodeGroupIDs: []int64{1},
		SubscriptionProviderConfig: `{
			"internal_defaults": {
				"interval": 900,
				"path": "./providers/{key}.yaml",
				"health-check": {"enable": true, "url": "https://probe.example.com/204", "interval": 600}
			},
			"all_nodes_provider": {"path": "./providers/all-nodes.yaml"}
		}`,
	}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	// 自动生成的组 provider：默认参数生效，{key} 被替换
	groupDef := findProviderDef(defs, "group-1")
	if groupDef == nil {
		t.Fatalf("自动内部 provider 缺失：%+v", defs)
	}
	if groupDef["interval"] != 900 {
		t.Fatalf("internal_defaults.interval 未生效：%v", groupDef["interval"])
	}
	if groupDef["path"] != "./providers/group-1.yaml" {
		t.Fatalf("path 模板 {key} 未替换：%v", groupDef["path"])
	}

	if findProviderDef(defs, "subscription-nodes") != nil {
		t.Fatalf("subscription-nodes 不应再输出：%+v", defs)
	}
}

// 显式 internal_providers 只需写要覆盖的字段，其余继承 internal_defaults。
func TestInternalProviderOverridesDefaults(t *testing.T) {
	tmpl := &entity.PlanTemplate{
		NodeGroupIDs: []int64{1},
		SubscriptionProviderConfig: `{
			"internal_defaults": {
				"interval": 1800,
				"path": "./providers/{key}.yaml",
				"health-check": {"enable": true, "url": "https://probe.example.com/204", "interval": 1200}
			},
			"internal_providers": {
				"jp": {"node_group_id": 1, "interval": 300}
			}
		}`,
	}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	jp := findProviderDef(defs, "jp")
	if jp == nil {
		t.Fatalf("显式 provider jp 缺失：%+v", defs)
	}
	if jp["interval"] != 300 {
		t.Fatalf("显式 interval 应覆盖默认：%v", jp["interval"])
	}
	if jp["path"] != "./providers/jp.yaml" {
		t.Fatalf("未显式配置的 path 应继承默认模板：%v", jp["path"])
	}
	hc, _ := jp["health-check"].(map[string]interface{})
	if hc["interval"] != float64(1200) {
		t.Fatalf("未显式配置的 health-check 应继承默认：%+v", hc)
	}
}

// 不配置任何默认参数时，组 provider 输出保持历史默认（3600 / ./providers/<key>.yaml），
// 且不会出现 subscription-nodes。
func TestProviderDefaultsFallbackUnchanged(t *testing.T) {
	tmpl := &entity.PlanTemplate{NodeGroupIDs: []int64{1}}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	groupDef := findProviderDef(defs, "group-1")
	if groupDef["interval"] != 3600 || groupDef["path"] != "./providers/group-1.yaml" {
		t.Fatalf("默认值不应改变：%+v", groupDef)
	}
	if findProviderDef(defs, "subscription-nodes") != nil {
		t.Fatalf("subscription-nodes 不应再输出：%+v", defs)
	}
}

// 外部 provider 的缓存路径不单独配置：与内部 provider 一致按默认模板生成，
// 手写的历史 path 会被忽略；health-check 原样透传。
func TestExternalProviderPathFollowsInternalTemplate(t *testing.T) {
	tmpl := &entity.PlanTemplate{
		NodeGroupIDs: []int64{1},
		SubscriptionProviderConfig: `{
			"internal_defaults": {"path": "./providers/{key}.yaml"},
			"external_providers": {
				"singbox": {
					"type": "http",
					"url": "https://example.com/oneclash.yaml",
					"path": "./profiles/proxies/mihomo.yaml",
					"interval": 1800,
					"health-check": {"enable": true, "url": "https://cp.cloudflare.com/generate_204", "interval": 3600}
				}
			}
		}`,
	}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	ext := findProviderDef(defs, "singbox")
	if ext == nil {
		t.Fatalf("外部 provider 缺失：%+v", defs)
	}
	if ext["path"] != "./providers/singbox.yaml" {
		t.Fatalf("外部 provider 的 path 应按默认模板生成，实际：%v", ext["path"])
	}
	hc, _ := ext["health-check"].(map[string]interface{})
	if hc["url"] != "https://cp.cloudflare.com/generate_204" || hc["interval"] != float64(3600) {
		t.Fatalf("外部 provider health-check 应原样保留：%+v", hc)
	}
}

// 没有配置路径模板时，外部 provider 回落到 ./providers/<key>.yaml。
func TestExternalProviderPathFallback(t *testing.T) {
	tmpl := &entity.PlanTemplate{
		NodeGroupIDs: []int64{1},
		SubscriptionProviderConfig: `{
			"external_providers": {
				"singbox": {
					"type": "http",
					"url": "https://example.com/oneclash.yaml",
					"path": "./profiles/proxies/mihomo.yaml"
				}
			}
		}`,
	}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	ext := findProviderDef(defs, "singbox")
	if ext == nil {
		t.Fatalf("外部 provider 缺失：%+v", defs)
	}
	if ext["path"] != "./providers/singbox.yaml" {
		t.Fatalf("外部 provider 的 path 应回落到默认值，实际：%v", ext["path"])
	}
}
