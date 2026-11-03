package providercfg

import "testing"

func TestParseAndValidate(t *testing.T) {
	raw := `{
		"internal_providers": {"office": {"node_group_id": 12, "interval": 3600}},
		"external_providers": {"singbox": {"type": "http", "url": "https://example.com/oneclash.yaml"}},
		"group_use": {"✈️ 墙外网站": ["office", "singbox"]}
	}`
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.InternalProviders["office"].NodeGroupID != 12 {
		t.Fatalf("internal provider should parse node_group_id: %+v", cfg.InternalProviders)
	}
	if err := Validate(raw); err != nil {
		t.Fatalf("validate: %v", err)
	}

	for _, bad := range []string{
		`{"internal_providers":{"subscription-nodes":{"node_group_id":1}}}`,
		`{"internal_providers":{"office":{}}}`,
		`{"external_providers":{"singbox":{"type":"http"}}}`,
		`{"group_use":{"":["office"]}}`,
		`not-json`,
	} {
		if err := Validate(bad); err == nil {
			t.Fatalf("expected invalid config: %s", bad)
		}
	}
}

func TestSanitizeKey(t *testing.T) {
	cases := map[string]string{
		"office":             "office",
		"  HK 01 ":           "HK-01",
		"✈️ 墙外网站":            "✈️-墙外网站",
		"a/b:c":              "a-b-c",
		"subscription-nodes": "subscription-nodes",
	}
	for input, want := range cases {
		if got := SanitizeKey(input); got != want {
			t.Fatalf("SanitizeKey(%q) = %q, want %q", input, got, want)
		}
	}
}
