package waf

import "testing"

func TestDefaultRuleFilesIncludesComboJSONL(t *testing.T) {
	var found *githubRuleDefault
	for i := range defaultRuleFiles {
		if defaultRuleFiles[i].Name == "combo.jsonl" {
			found = &defaultRuleFiles[i]
			break
		}
	}
	if found == nil {
		t.Fatal("defaultRuleFiles must include combo.jsonl (三层规则引擎的 L2 规则集)")
	}
	if found.MergeMode != "replace" {
		t.Fatalf("combo.jsonl merge mode = %q, want replace", found.MergeMode)
	}
}

func TestDefaultGitHubSourceIncludesComboJSONL(t *testing.T) {
	src := DefaultGitHubSource()
	for _, fm := range src.FileMappings {
		if fm.Name == "combo.jsonl" {
			return
		}
	}
	t.Fatal("DefaultGitHubSource must include combo.jsonl")
}

func TestMissingDefaultRuleFilesFillsLegacyManifest(t *testing.T) {
	// 旧版本保存的同步清单（没有 combo.jsonl）
	legacy := []string{
		"args.rule", "cookie.rule", "fileext.rule", "header.rule",
		"post.rule", "referer.rule", "url.rule", "useragent.rule", "whiteua.rule",
	}
	missing := missingDefaultRuleFiles(legacy)
	if len(missing) != 1 || missing[0].Name != "combo.jsonl" {
		t.Fatalf("missing = %+v, want only combo.jsonl", missing)
	}
	if got := missingDefaultRuleFiles([]string{"combo.jsonl"}); len(got) != len(defaultRuleFiles)-1 {
		t.Fatalf("missing count = %d, want %d", len(got), len(defaultRuleFiles)-1)
	}
}

func TestIsBuiltinGitHubSource(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{githubRawBase, true},
		{githubRawBase + "/", true},
		{" " + githubRawBase + " ", true},
		{"https://example.com/nginxguard/rule-config", false},
		{"https://raw.githubusercontent.com/someone/nginxguard/main/rule-config", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isBuiltinGitHubSource(c.url); got != c.want {
			t.Errorf("isBuiltinGitHubSource(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
