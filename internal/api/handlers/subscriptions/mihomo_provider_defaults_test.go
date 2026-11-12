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

// internal_defaults 要能控制自动生成的内部 provider 与保留的 subscription-nodes：
// path 支持 {key} 占位，多 provider 不会互相覆盖；health-check 可换成自定义探测地址。
func TestProviderDefaultsApplyToInternalAndAllNodes(t *testing.T) {
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

	// 保留的 subscription-nodes：覆盖 path，interval/health-check 继承默认
	allDef := findProviderDef(defs, "subscription-nodes")
	if allDef == nil {
		t.Fatalf("subscription-nodes 缺失：%+v", defs)
	}
	if allDef["path"] != "./providers/all-nodes.yaml" {
		t.Fatalf("all_nodes_provider.path 未生效：%v", allDef["path"])
	}
	if allDef["interval"] != 900 {
		t.Fatalf("subscription-nodes 应继承 internal_defaults.interval：%v", allDef["interval"])
	}
	hc, _ := allDef["health-check"].(map[string]interface{})
	if hc["url"] != "https://probe.example.com/204" || hc["interval"] != float64(600) {
		t.Fatalf("subscription-nodes 应继承 internal_defaults.health-check：%+v", hc)
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

// 不配置任何默认参数时，输出保持历史默认（3600 / ./providers/<key>.yaml）。
func TestProviderDefaultsFallbackUnchanged(t *testing.T) {
	tmpl := &entity.PlanTemplate{NodeGroupIDs: []int64{1}}
	h := &PublicHandler{}
	plan := h.resolveSubscriptionProviderPlan(context.Background(), tmpl, nil, nil)
	defs := h.buildProviderDefinitions(providerTestContext(), "tok", plan)

	allDef := findProviderDef(defs, "subscription-nodes")
	if allDef["interval"] != 3600 || allDef["path"] != "./providers/subscription-nodes.yaml" {
		t.Fatalf("默认值不应改变：%+v", allDef)
	}
}
