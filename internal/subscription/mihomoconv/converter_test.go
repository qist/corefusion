package mihomoconv

import (
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
