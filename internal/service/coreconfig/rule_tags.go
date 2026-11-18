package coreconfig

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"strings"
)

// ruleTagAlphabet 与前端 genTag() 的 Math.random().toString(36) 字符集一致。
const ruleTagAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// NewRuleTag 生成与前端同格式的 xray 路由规则名：rule-xxxxxx。
// 后端写入的规则（地区导入、默认基线）必须自带名字，否则路由页的
// 「规则名称」列是空的，用户点开编辑再保存才会由前端临时生成一个。
func NewRuleTag() string {
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(ruleTagAlphabet))))
		if err != nil {
			b[i] = '0'
			continue
		}
		b[i] = ruleTagAlphabet[n.Int64()]
	}
	return "rule-" + string(b)
}

// BackfillXrayRuleTags 给该节点 xray 路由里缺少 ruleTag 的规则补名字。
// 幂等：已有名字的规则原样保留；返回补了多少条。
func (s *Service) BackfillXrayRuleTags(ctx context.Context, serverID int64) (int, error) {
	raw, err := s.GetSection(ctx, serverID, "xray", "routing")
	if err != nil {
		return 0, err
	}
	routing, ok := raw.(map[string]interface{})
	if !ok || routing == nil {
		return 0, nil
	}
	rules, _ := routing["rules"].([]interface{})
	if len(rules) == 0 {
		return 0, nil
	}

	used := make(map[string]bool, len(rules))
	for _, item := range rules {
		rule, _ := item.(map[string]interface{})
		if rule == nil {
			continue
		}
		if tag, _ := rule["ruleTag"].(string); strings.TrimSpace(tag) != "" {
			used[tag] = true
		}
	}

	added := 0
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok || rule == nil {
			continue
		}
		if tag, _ := rule["ruleTag"].(string); strings.TrimSpace(tag) != "" {
			continue
		}
		tag := NewRuleTag()
		for used[tag] {
			tag = NewRuleTag()
		}
		rule["ruleTag"] = tag
		used[tag] = true
		added++
	}
	if added == 0 {
		return 0, nil
	}

	routing["rules"] = rules
	data, err := json.Marshal(routing)
	if err != nil {
		return 0, err
	}
	if err := s.SaveSection(ctx, serverID, "xray", "routing", data); err != nil {
		return 0, err
	}
	return added, nil
}
