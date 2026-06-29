package formatter

import (
	"fmt"
	"strings"
	"testing"
)

func TestURIToMihomo(t *testing.T) {
	uri := "vless://ec5d2637-19af-45db-94e0-8f97b0ac88e6@192.168.100.128:20296?type=raw&encryption=none&headerType=http&path=%2F&host=www.baidu.com&security=none#inbound-vless-20296"
	node := ParseURIToNode(uri)
	if node == nil {
		t.Fatal("parse failed")
	}
	fmt.Printf("Protocol: %s\n", node.Protocol)
	fmt.Printf("Transport: %s\n", node.Transport)
	fmt.Printf("TLS: %v\n", node.TLS)
	fmt.Printf("Extra: %v\n", node.Extra)
	fmt.Println()
	fmt.Println("=== Mihomo YAML ===")
	fmt.Println(URIToMihomo(uri))
}

func TestMihomoFormatterUsesHTTPNetworkWhenHTTPOptsPresent(t *testing.T) {
	f := &MihomoFormatter{}
	data, err := f.Format([]Node{
		{
			Name:      "node-a",
			Protocol:  "vless",
			Host:      "example.com",
			Port:      443,
			UUID:      "11111111-1111-1111-1111-111111111111",
			Transport: "raw",
			Extra: map[string]interface{}{
				"http-opts": map[string]interface{}{
					"path": []string{"/"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("format mihomo yaml: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "network: http") {
		t.Fatalf("expected network: http in yaml, got %q", text)
	}
}
