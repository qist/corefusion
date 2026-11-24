package waf

import (
	"context"
	"encoding/json"
	"testing"
)

func mappingNames(cfg *GitHubSourceConfig) []string {
	names := make([]string, 0, len(cfg.FileMappings))
	for _, fm := range cfg.FileMappings {
		names = append(names, fm.Name)
	}
	return names
}

// 历史保存的同步清单里没有后来新增的引擎文件（combo.jsonl）。
// 配置接口必须返回补全后的集合，否则「全局规则里有 combo.jsonl、规则源里看不到」。
func TestGetGitHubSourceFillsMissingBuiltinFiles(t *testing.T) {
	legacy := GitHubSourceConfig{
		FileMappings: []SourceFileMapping{
			{Name: "args.rule", URL: "https://raw.githubusercontent.com/qist/nginxguard/main/rule-config/args.rule"},
			{Name: "cookie.rule"},
			{Name: "fileext.rule"},
			{Name: "header.rule"},
			{Name: "post.rule"},
			{Name: "referer.rule"},
			{Name: "url.rule"},
			{Name: "useragent.rule"},
			{Name: "whiteua.rule"},
		},
		SyncCron: "@every 1h",
		Enabled:  true,
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{settingSvc: &fakeSettingStore{vals: map[string]string{"waf.github_source": string(raw)}}}

	cfg, err := svc.GetGitHubSource(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fm := range cfg.FileMappings {
		if fm.Name == "combo.jsonl" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("内置源应补全 combo.jsonl，实际 %v", mappingNames(cfg))
	}
	if len(cfg.FileMappings) != len(legacy.FileMappings)+1 {
		t.Fatalf("映射数 = %d，期望 %d", len(cfg.FileMappings), len(legacy.FileMappings)+1)
	}
}

// 自定义镜像源不做补全：它可能没有 combo.jsonl，补进去会让同步反复 404。
func TestGetGitHubSourceKeepsCustomSourceUntouched(t *testing.T) {
	custom := GitHubSourceConfig{
		URL:          "https://example.com/nginxguard/rule-config",
		FileMappings: []SourceFileMapping{{Name: "args.rule"}},
		SyncCron:     "@every 6h",
		Enabled:      true,
	}
	raw, err := json.Marshal(custom)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{settingSvc: &fakeSettingStore{vals: map[string]string{"waf.github_source": string(raw)}}}

	cfg, err := svc.GetGitHubSource(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.FileMappings) != 1 || cfg.FileMappings[0].Name != "args.rule" {
		t.Fatalf("自定义源不应被补全，实际 %v", mappingNames(cfg))
	}
}
