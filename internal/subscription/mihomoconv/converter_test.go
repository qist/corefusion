package mihomoconv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
)

func TestConvertsV2Ray(t *testing.T) {
	uri := "vless://ec5d2637-19af-45db-94e0-8f97b0ac88e6@192.168.100.128:20296?encryption=none&headerType=http&host=www.baidu.com&path=%2F&security=none&type=raw#inbound-vless-20296"
	proxies, err := ConvertsV2Ray([]byte(uri))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range proxies {
		fmt.Printf("name: %v\n", p["name"])
		fmt.Printf("network: %v\n", p["network"])
		fmt.Printf("http-opts: %v\n", p["http-opts"])
	}
}

// 裸 TCP（链接里的 type 是 ""/tcp/raw 三者之一）在 mihomo 侧**不写 network**：
// mihomo 的 network 取值只有 ws/http/h2/grpc/httpupgrade/xhttp（裸 TCP 就是省略），
// 写 `network: tcp`/`raw` 属于客户端不认识的值 —— 但也不能因此改成别的传输（降级）。
func TestBareTCPOmitsMihomoNetwork(t *testing.T) {
	assertNoNetwork := func(t *testing.T, raw []byte, label string) {
		t.Helper()
		proxies, err := ConvertsV2Ray(raw)
		if err != nil || len(proxies) != 1 {
			t.Fatalf("%s 转换失败: %v (%d)", label, err, len(proxies))
		}
		if v, ok := proxies[0]["network"]; ok {
			t.Errorf("%s network = %v, want 不写该字段", label, v)
		}
	}

	assertNoNetwork(t, []byte("vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&security=none&type=raw#n1"), "vless type=raw")
	assertNoNetwork(t, []byte("vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&security=none#n1b"), "vless 无 type")
	assertNoNetwork(t, []byte("trojan://pw@1.2.3.4:443?security=tls&type=raw#n2"), "trojan type=raw")

	body, _ := json.Marshal(map[string]interface{}{
		"v": "2", "ps": "n3", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0, "net": "raw",
	})
	assertNoNetwork(t, []byte("vmess://"+base64.StdEncoding.EncodeToString(body)), "vmess net=raw")

	// 真正的传输照旧保留（不许被"当成 TCP"抹掉）
	proxies, err := ConvertsV2Ray([]byte("vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&security=none&type=ws&path=%2Fws#n4"))
	if err != nil || len(proxies) != 1 {
		t.Fatalf("ws 转换失败: %v", err)
	}
	if got := proxies[0]["network"]; got != "ws" {
		t.Errorf("ws network = %v, want ws", got)
	}
	// raw + headerType=http 是 mihomo 的 http 传输（要保留）
	proxies, err = ConvertsV2Ray([]byte("vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&security=none&type=raw&headerType=http&host=www.a.com#n5"))
	if err != nil || len(proxies) != 1 {
		t.Fatalf("raw+http 伪装转换失败: %v", err)
	}
	if got := proxies[0]["network"]; got != "http" {
		t.Errorf("raw+headerType=http 的 network = %v, want http", got)
	}
	if _, ok := proxies[0]["http-opts"]; !ok {
		t.Errorf("raw+headerType=http 应带 http-opts: %v", proxies[0])
	}
}
