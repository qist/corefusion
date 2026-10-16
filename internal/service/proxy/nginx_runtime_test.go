package proxy

import "testing"

// 面板托管的 nginx：进程标题里的二进制与 `-c` 都是相对 dataDir 的路径 ✓
// （`exec.Command(relBin, "-c", "proxy/nginx.conf")` + `cmd.Dir = dataDir` ✓），
// 必须按进程 cwd 解析成绝对路径 ✓。
func TestParseNginxMasterCmdline_ManagedRelative(t *testing.T) {
	bin, conf, ok := parseNginxMasterCmdline(
		"nginx: master process binaries/proxy/nginx-linux-amd64 -c proxy/nginx.conf",
		"/opt/corefusion/data",
	)
	if !ok {
		t.Fatal("托管的 nginx master 应被识别")
	}
	if bin != "/opt/corefusion/data/binaries/proxy/nginx-linux-amd64" {
		t.Errorf("bin = %q", bin)
	}
	if conf != "/opt/corefusion/data/proxy/nginx.conf" {
		t.Errorf("conf = %q", conf)
	}
}

// 外部 nginx（真机 /apps 与 /etc/nginx 之争 ✓）：二进制与配置文件都是绝对路径 ✓，
// 不能被改写成 dataDir/proxy/nginx.conf ✗。
func TestParseNginxMasterCmdline_ExternalAbsolute(t *testing.T) {
	bin, conf, ok := parseNginxMasterCmdline(
		"nginx: master process /apps/nginx/sbin/nginx -c /etc/nginx/nginx.conf",
		"/opt/corefusion-agent/data",
	)
	if !ok {
		t.Fatal("外部 nginx master 应被识别")
	}
	if bin != "/apps/nginx/sbin/nginx" {
		t.Errorf("bin = %q", bin)
	}
	if conf != "/etc/nginx/nginx.conf" {
		t.Errorf("conf = %q", conf)
	}
}

// 编译默认配置启动（无 `-c`）✓：仍识别为 nginx，但 conf 为空 ✓，
// 由上层用 `nginx -V --conf-path=` 兜底 ✓。
func TestParseNginxMasterCmdline_NoConfFlag(t *testing.T) {
	bin, conf, ok := parseNginxMasterCmdline("nginx: master process /usr/sbin/nginx", "/")
	if !ok {
		t.Fatal("应识别为 nginx master")
	}
	if bin != "/usr/sbin/nginx" || conf != "" {
		t.Errorf("bin=%q conf=%q", bin, conf)
	}
}

// worker 及其它进程不能被误判成 master ✓。
func TestParseNginxMasterCmdline_NotMaster(t *testing.T) {
	for _, cmdline := range []string{
		"nginx: worker process",
		"nginx: cache manager process",
		"/usr/bin/caddy run --config Caddyfile",
		"",
	} {
		if _, _, ok := parseNginxMasterCmdline(cmdline, "/"); ok {
			t.Errorf("不应识别：%q", cmdline)
		}
	}
}
