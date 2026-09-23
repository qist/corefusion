package certificates

import (
	"archive/zip"
	"bytes"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	rtcert "corefusion/internal/runtime/cert"
	certService "corefusion/internal/service/certificate"
)

func mkBlock(marker string) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte(marker)})
}

func sampleMaterial(name string) *certService.CertMaterial {
	full := bytes.Join([][]byte{mkBlock("leaf"), mkBlock("intermediate")}, nil)
	return &certService.CertMaterial{
		Name:      name,
		Domain:    name,
		Issuer:    "Let's Encrypt",
		ExpiresAt: time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC),
		CertPEM:   string(full),
		KeyPEM:    "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n",
	}
}

func readZip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip 打不开 ✗：%v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, oerr := f.Open()
		if oerr != nil {
			t.Fatal(oerr)
		}
		var buf bytes.Buffer
		if _, cerr := buf.ReadFrom(rc); cerr != nil {
			t.Fatal(cerr)
		}
		_ = rc.Close()
		out[f.Name] = buf.String()
	}
	return out
}

// 单个证书：根目录三个文件 ✓；cert.pem **仅叶**（1 段 ✓）、fullchain 含链（2 段 ✓）
func TestBuildCertZipSingle(t *testing.T) {
	data, err := buildCertZip([]certZipItem{{Mat: sampleMaterial("a.example.com")}})
	if err != nil {
		t.Fatal(err)
	}
	files := readZip(t, data)
	for _, want := range []string{zipFileNameCert, zipFileNameFullchain, zipFileNameKey, zipFileNameReadme} {
		if _, ok := files[want]; !ok {
			t.Fatalf("缺少 %s ✓（实际 %v）", want, keys(files))
		}
	}
	if got := rtcert.CountCertificateBlocks([]byte(files[zipFileNameCert])); got != 1 {
		t.Fatalf("包里的 cert.pem 应**仅叶证书** ✓，实际 %d 段", got)
	}
	if got := rtcert.CountCertificateBlocks([]byte(files[zipFileNameFullchain])); got != 2 {
		t.Fatalf("包里的 fullchain.pem 应含中间链 ✓，实际 %d 段", got)
	}
	if !strings.Contains(files[zipFileNameKey], "PRIVATE KEY") {
		t.Fatalf("私钥文件内容不对 ✗")
	}
	// README 必须讲清"两个 cert.pem 的区别" ✓（这正是用户会问的点 ✓）
	if !strings.Contains(files[zipFileNameReadme], "fullchain.pem") || !strings.Contains(files[zipFileNameReadme], "含完整链") {
		t.Fatalf("README 应说明差异 ✓：%s", files[zipFileNameReadme][:min(120, len(files[zipFileNameReadme]))])
	}
}

// 批量：每个证书一个目录 ✓ + 根目录清单 ✓；重名目录自动加序号 ✓
func TestBuildCertZipBatch(t *testing.T) {
	data, err := buildCertZip([]certZipItem{
		{Folder: "a.example.com", Mat: sampleMaterial("a.example.com")},
		{Folder: "b.example.com", Mat: sampleMaterial("b.example.com")},
		{Folder: "b.example.com-2", Mat: sampleMaterial("b.example.com")},
	})
	if err != nil {
		t.Fatal(err)
	}
	files := readZip(t, data)
	if _, ok := files[zipFileNameReadme]; !ok {
		t.Fatalf("批量包应有根清单 ✓（实际 %v）", keys(files))
	}
	for _, p := range []string{
		"a.example.com/" + zipFileNameCert,
		"a.example.com/" + zipFileNameKey,
		"b.example.com/" + zipFileNameFullchain,
		"b.example.com-2/" + zipFileNameCert,
	} {
		if _, ok := files[p]; !ok {
			t.Fatalf("缺少 %s ✓（实际 %v）", p, keys(files))
		}
	}
	// 每个目录里的 README 要带上自己的域名/到期 ✓
	if !strings.Contains(files["a.example.com/"+zipFileNameReadme], "a.example.com") {
		t.Fatalf("目录内 README 应含域名 ✓")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
