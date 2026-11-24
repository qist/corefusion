package dnsrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// ─────────── GoDaddy ───────────

func TestGoDaddyUpsertAndDelete(t *testing.T) {
	var mu sync.Mutex
	var puts, deletes int
	var lastBody string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/domains/tycng.com", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/domains/tycng.com/records/A/entry", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			puts++
			b, _ := io.ReadAll(r.Body)
			lastBody = string(b)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	up, err := newUpdater("godaddy", map[string]string{
		"GODADDY_API_KEY": "k", "GODADDY_API_SECRET": "s",
	}, ProviderConfig{HTTPClient: srv.Client(), APIBase: srv.URL})
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}

	ctx := context.Background()
	if err := up.Upsert(ctx, Record{Type: "A", Name: "entry.tycng.com", Content: "1.2.3.4", TTL: 60}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if puts != 1 {
		t.Fatalf("应 PUT 一次，实际 %d", puts)
	}
	var body []map[string]any
	if err := json.Unmarshal([]byte(lastBody), &body); err != nil {
		t.Fatalf("body 非法：%s", lastBody)
	}
	if len(body) != 1 || body[0]["data"] != "1.2.3.4" || body[0]["name"] != "entry" {
		t.Fatalf("请求体不符：%s", lastBody)
	}
	if ttl, _ := body[0]["ttl"].(float64); ttl < 600 {
		t.Errorf("GoDaddy TTL 应 >= 600，实际 %v", body[0]["ttl"])
	}

	if err := up.Delete(ctx, "entry.tycng.com", "A"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deletes != 1 {
		t.Fatalf("应 DELETE 一次，实际 %d", deletes)
	}
}

// ─────────── DNSPod ───────────

func TestDNSPodUpsertModifiesExistingRecord(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	var modifyForm url.Values

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := strings.TrimPrefix(r.URL.Path, "/")
		mu.Lock()
		actions = append(actions, action)
		mu.Unlock()
		switch action {
		case "Domain.List":
			fmt.Fprint(w, `{"status":{"code":"1"},"domains":[{"name":"tycng.com"}]}`)
		case "Record.List":
			fmt.Fprint(w, `{"status":{"code":"1"},"records":[{"id":"r1","name":"entry","type":"A","value":"9.9.9.9"}]}`)
		case "Record.Modify":
			mu.Lock()
			modifyForm = r.PostForm
			mu.Unlock()
			fmt.Fprint(w, `{"status":{"code":"1"}}`)
		case "Record.Create":
			fmt.Fprint(w, `{"status":{"code":"1"}}`)
		case "Record.Remove":
			fmt.Fprint(w, `{"status":{"code":"1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	up, err := newUpdater("dnspod", map[string]string{"DNSPOD_API_KEY": "12345,token"}, ProviderConfig{HTTPClient: srv.Client(), APIBase: srv.URL})
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}
	if err := up.Upsert(context.Background(), Record{Type: "A", Name: "entry.tycng.com", Content: "1.2.3.4", TTL: 600}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if modifyForm == nil {
		t.Fatalf("已存在记录时应走 Modify，实际 actions=%v", actions)
	}
	if got := modifyForm.Get("record_id"); got != "r1" {
		t.Errorf("record_id = %q, want r1", got)
	}
	if got := modifyForm.Get("value"); got != "1.2.3.4" {
		t.Errorf("value = %q", got)
	}
	if got := modifyForm.Get("sub_domain"); got != "entry" {
		t.Errorf("sub_domain = %q", got)
	}
}

// DNSPod 的 Record.List 在「无记录」时返回 code=10，应按空集处理（走创建）。
func TestDNSPodNoRecordCreates(t *testing.T) {
	var mu sync.Mutex
	created := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch strings.TrimPrefix(r.URL.Path, "/") {
		case "Domain.List":
			fmt.Fprint(w, `{"status":{"code":"1"},"domains":[{"name":"tycng.com"}]}`)
		case "Record.List":
			fmt.Fprint(w, `{"status":{"code":"10","message":"No records"}}`)
		case "Record.Create":
			mu.Lock()
			created = true
			mu.Unlock()
			fmt.Fprint(w, `{"status":{"code":"1"}}`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	up, err := newUpdater("dnspod", map[string]string{"DNSPOD_API_KEY": "1,t"}, ProviderConfig{HTTPClient: srv.Client(), APIBase: srv.URL})
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}
	if err := up.Upsert(context.Background(), Record{Type: "A", Name: "entry.tycng.com", Content: "1.2.3.4"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !created {
		t.Fatal("无记录时应创建")
	}
}

// ─────────── Namecheap ───────────

func TestNamecheapUpsertRewritesWholeHostList(t *testing.T) {
	var mu sync.Mutex
	var setForm url.Values

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("Command")
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			cmd = r.PostForm.Get("Command")
			if cmd == "namecheap.domains.dns.setHosts" {
				mu.Lock()
				setForm = r.PostForm
				mu.Unlock()
			}
		}
		switch cmd {
		case "namecheap.domains.dns.getHosts":
			fmt.Fprint(w, `<?xml version="1.0"?>
<ApiResponse Status="OK"><CommandResponse Type="namecheap.domains.dns.getHosts">
<DomainDNSGetHostsResult Domain="tycng.com" IsUsingOurDNS="true">
<host HostId="1" Name="@" Type="A" Address="10.0.0.1" MXPref="10" TTL="1800"/>
<host HostId="2" Name="entry" Type="A" Address="9.9.9.9" MXPref="10" TTL="1800"/>
</DomainDNSGetHostsResult></CommandResponse></ApiResponse>`)
		case "namecheap.domains.dns.setHosts":
			fmt.Fprint(w, `<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse Type="namecheap.domains.dns.setHosts"/></ApiResponse>`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	up, err := newUpdater("namecheap", map[string]string{
		"NAMECHEAP_API_USER": "u", "NAMECHEAP_API_KEY": "k", "NAMECHEAP_CLIENT_IP": "1.2.3.4",
	}, ProviderConfig{HTTPClient: srv.Client(), APIBase: srv.URL})
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}
	if err := up.Upsert(context.Background(), Record{Type: "A", Name: "entry.tycng.com", Content: "1.2.3.4"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if setForm == nil {
		t.Fatal("应调用 setHosts")
	}
	// 根域名记录必须原样保留，目标记录被改写。
	var seenRoot, seenEntry bool
	for i := 1; i <= 5; i++ {
		idx := fmt.Sprint(i)
		name := setForm.Get("HostName" + idx)
		if name == "" {
			break
		}
		addr := setForm.Get("Address" + idx)
		switch name {
		case "@":
			seenRoot = addr == "10.0.0.1"
		case "entry":
			seenEntry = addr == "1.2.3.4"
		}
	}
	if !seenRoot {
		t.Error("根域名 A 记录丢了或被改坏")
	}
	if !seenEntry {
		t.Error("目标记录未改成新值")
	}
}

// ─────────── 工具函数 ───────────

func TestSplitDomainTLD(t *testing.T) {
	cases := []struct{ in, sld, tld string }{
		{"tycng.com", "tycng", "com"},
		{"example.co.uk", "example", "co.uk"},
		{"a.example.com.cn", "a", "example.com.cn"},
	}
	for _, c := range cases {
		sld, tld := splitDomainTLD(c.in)
		if sld != c.sld || tld != c.tld {
			t.Errorf("splitDomainTLD(%q)=(%q,%q), want (%q,%q)", c.in, sld, tld, c.sld, c.tld)
		}
	}
}

func TestRecordValueQuoting(t *testing.T) {
	if got := route53Value("TXT", "abc"); got != `"abc"` {
		t.Errorf("route53 TXT 应加引号：%q", got)
	}
	if got := googleRecordValue("TXT", "abc"); got != `"abc"` {
		t.Errorf("google TXT 应加引号：%q", got)
	}
	if got := huaweiRecordValue("TXT", "abc"); got != `"abc"` {
		t.Errorf("huawei TXT 应加引号：%q", got)
	}
	if got := route53Value("A", "1.2.3.4"); got != "1.2.3.4" {
		t.Errorf("A 记录不应改值：%q", got)
	}
}

// 六个新服务商都必须已注册（能力表里 implemented=true）。
func TestAllProvidersRegistered(t *testing.T) {
	for _, p := range []string{"cloudflare", "alidns", "dnspod", "godaddy", "namecheap", "route53", "google", "huawei"} {
		if _, ok := updaterFactories[p]; !ok {
			t.Errorf("%s 未注册", p)
		}
	}
	caps := Capabilities(nil)
	for _, c := range caps {
		if !c.Implemented {
			t.Errorf("%s 应标记 implemented", c.Provider)
		}
		if c.Provider != "cloudflare" && c.Verified {
			t.Errorf("%s 未实测，不应标记 verified", c.Provider)
		}
	}
}
