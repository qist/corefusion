package certificates

import (
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 接口出参：① 算出 has_content ✓；② **绝不回传 PEM/私钥** ✗。
func TestStripDataPrefixMetaAndNoKeyLeak(t *testing.T) {
	certs := []entity.Certificate{
		{
			ID:          1,
			Name:        "panel.example.com",
			CertPath:    "data/certs/panel.example.com/cert.pem",
			KeyPath:     "data/certs/panel.example.com/key.pem",
			CertContent: "-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----\n",
			KeyContent:  "-----BEGIN PRIVATE KEY-----\nSECRET\n-----END PRIVATE KEY-----\n",
		},
		{
			// 节点上报的证书：面板只留元数据 ✓（没有内容 ✓）
			ID:       2,
			Name:     "node.example.com",
			CertPath: "certs/node.example.com/cert.pem",
			KeyPath:  "certs/node.example.com/key.pem",
		},
	}
	stripDataPrefix(certs)

	if !certs[0].HasContent {
		t.Fatalf("有 PEM 的证书 has_content 应为 true ✓")
	}
	if certs[1].HasContent {
		t.Fatalf("节点持有的证书 has_content 应为 false ✓")
	}
	// 路径前缀照旧被剥掉 ✓
	if strings.HasPrefix(certs[0].CertPath, "data/") {
		t.Fatalf("data/ 前缀应被剥掉 ✓：%q", certs[0].CertPath)
	}
	// 序列化后**不能**出现私钥 ✗（以前列表接口会把 key_content 原样回给前端 ✗）
	out, err := json.Marshal(certs)
	if err != nil {
		t.Fatal(err)
	}
	// ⚠ 断言的是**值**为空 ✗，不是字段名消失 ✓（字段仍然存在 ✓，只是内容已清空 ✓）
	if strings.Contains(string(out), "SECRET") || strings.Contains(string(out), "AAA") {
		t.Fatalf("接口出参泄漏了 PEM/私钥内容 ✗：%s", string(out))
	}
	if !strings.Contains(string(out), `"key_content":""`) || !strings.Contains(string(out), `"cert_content":""`) {
		t.Fatalf("内容字段应为空串 ✓：%s", string(out))
	}
	if !strings.Contains(string(out), `"has_content":true`) || !strings.Contains(string(out), `"has_content":false`) {
		t.Fatalf("has_content 没进 JSON ✓：%s", string(out))
	}
}
