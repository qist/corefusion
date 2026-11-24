package subscriptions

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// mihomo 的逻辑规则（AND/OR/NOT）条件里带逗号和括号，订阅生成必须原样输出，
// 否则客户端会拿到被截断的规则（例如 AND,((NETWORK,UDP) 这种半截语法）。
//
// 现实用例：AND,((NETWORK,UDP),(DST-PORT,3478/19302)),REJECT
func TestGenerateMihomoYAMLKeepsLogicalRules(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{}`}
	rules := []entity.MihomoRule{
		{RuleString: "AND,((NETWORK,UDP),(DST-PORT,3478/19302)),REJECT", Enabled: true},
		{RuleString: "OR,((DOMAIN-SUFFIX,a.com),(DOMAIN-SUFFIX,b.com)),PROXY", Enabled: true},
		{RuleString: "NOT,((NETWORK,UDP)),DIRECT", Enabled: true},
		{RuleString: "SUB-RULE,((NETWORK,tcp),(SUB-RULE,(ads))),REJECT", Enabled: true},
		// 新版 mihomo 的其它规则类型同样原样输出（来源 IP / 入站 / 进程正则 / UID / DSCP）。
		{RuleString: "SRC-IP-CIDR,192.168.1.0/24,DIRECT", Enabled: true},
		{RuleString: "SRC-GEOIP,cn,DIRECT", Enabled: true},
		{RuleString: "IN-TYPE,SOCKS/HTTP,PROXY", Enabled: true},
		{RuleString: "IN-USER,mihomo,PROXY", Enabled: true},
		{RuleString: "PROCESS-NAME-REGEX,curl$,PROXY", Enabled: true},
		{RuleString: "PROCESS-PATH-WILDCARD,/usr/*/wget,PROXY", Enabled: true},
		{RuleString: "IP-SUFFIX,8.8.8.8/24,PROXY", Enabled: true},
		{RuleString: "UID,1001,DIRECT", Enabled: true},
		{RuleString: "DSCP,4,DIRECT", Enabled: true},
		{RuleString: "MATCH,DIRECT", Enabled: true},
	}

	h := &PublicHandler{}
	out := string(h.generateMihomoYAML(cfg, rules, nil, nil))

	for _, want := range []string{
		"  - AND,((NETWORK,UDP),(DST-PORT,3478/19302)),REJECT\n",
		"  - OR,((DOMAIN-SUFFIX,a.com),(DOMAIN-SUFFIX,b.com)),PROXY\n",
		"  - NOT,((NETWORK,UDP)),DIRECT\n",
		"  - SUB-RULE,((NETWORK,tcp),(SUB-RULE,(ads))),REJECT\n",
		"  - SRC-IP-CIDR,192.168.1.0/24,DIRECT\n",
		"  - SRC-GEOIP,cn,DIRECT\n",
		"  - IN-TYPE,SOCKS/HTTP,PROXY\n",
		"  - IN-USER,mihomo,PROXY\n",
		"  - PROCESS-NAME-REGEX,curl$,PROXY\n",
		"  - PROCESS-PATH-WILDCARD,/usr/*/wget,PROXY\n",
		"  - IP-SUFFIX,8.8.8.8/24,PROXY\n",
		"  - UID,1001,DIRECT\n",
		"  - DSCP,4,DIRECT\n",
		"  - MATCH,DIRECT\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("生成的 YAML 缺少规则行：%q\n实际输出：\n%s", strings.TrimSpace(want), out)
		}
	}
}

// 禁用的逻辑规则不应出现在订阅里。
func TestGenerateMihomoYAMLSkipsDisabledLogicalRule(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{}`}
	rules := []entity.MihomoRule{
		{RuleString: "AND,((NETWORK,UDP),(DST-PORT,3478/19302)),REJECT", Enabled: false},
	}
	out := string((&PublicHandler{}).generateMihomoYAML(cfg, rules, nil, nil))
	if strings.Contains(out, "3478/19302") {
		t.Fatalf("禁用的规则不应输出：\n%s", out)
	}
}
