package mihomoconv

import "testing"

// worker 类链接的 path 里带 ?ed=2560：必须保留真实值，不能写死成 4096。
func TestConvertsV2RayKeepsEarlyDataValue(t *testing.T) {
	uri := "vless://d094c691-d4ec-4dd1-b935-83a764484716@maimai.boown.com:443" +
		"?encryption=none&security=tls&type=ws&host=maimai.boown.com&sni=maimai.boown.com" +
		"&fp=random&path=%2F%3Fed%3D2560#maimai.boown.com"
	proxies, err := ConvertsV2Ray([]byte(uri))
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("proxies = %d, want 1", len(proxies))
	}
	ws, ok := proxies[0]["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("ws-opts missing: %#v", proxies[0])
	}
	if got, _ := ws["max-early-data"].(int); got != 2560 {
		t.Fatalf("max-early-data = %v, want 2560", ws["max-early-data"])
	}
}
