package logmanage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type memSettingStore struct{ m map[string]string }

func (s *memSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	if v, ok := s.m[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func (s *memSettingStore) Set(_ context.Context, key, value string) error {
	s.m[key] = value
	return nil
}

// 老库里 mode=stdout + managed=true 的配置读取后必须变成 file + managed=false，
// 并写回数据库，避免每次读取都重复迁移、以及 UI 继续显示“标准输出”。
func TestGetLogOutputMigratesStdoutToFile(t *testing.T) {
	store := &memSettingStore{m: map[string]string{
		settingKeyLogOutput: `{"nginx":{"mode":"stdout","managed":true,"capture_path":"logs/nginx-stdout.log"},"caddy":{"mode":"off"}}`,
	}}
	svc := &Service{settingStore: store}

	cfg, err := svc.GetLogOutputForServer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Nginx.Mode != "file" || cfg.Nginx.Managed {
		t.Fatalf("nginx = %+v, want file & managed=false", cfg.Nginx)
	}
	if cfg.Caddy.Mode != "off" {
		t.Fatalf("caddy = %+v, want off", cfg.Caddy)
	}

	var saved LogOutputConfig
	if err := json.Unmarshal([]byte(store.m[settingKeyLogOutput]), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Nginx.Mode != "file" || saved.Nginx.Managed {
		t.Fatalf("persisted nginx = %+v, want file & managed=false", saved.Nginx)
	}
}

// 标准输出模式已下线：历史配置里的 stdout 必须归一为 file，
// 否则面板不会按文件采集/轮转，日志页仍然是空的。
func TestNormalizeLogModeRetiresStdout(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"stdout", "file"},
		{"file", "file"},
		{"", "file"},
		{"unknown", "file"},
		{"off", "off"},
	}
	for _, c := range cases {
		if got := normalizeLogMode(c.in); got != c.want {
			t.Errorf("normalizeLogMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// xray 的 access/error 为空或 /dev/stderr 在旧版本表示 stdout；
// 归一下不能再推导出 stdout。
func TestInferXrayLogModeNeverReturnsStdout(t *testing.T) {
	cases := []struct {
		name string
		log  map[string]interface{}
		want string
	}{
		{"stdout access", map[string]interface{}{"access": "", "error": "logs/xray-error.log", "loglevel": "info"}, "file"},
		{"stdout error", map[string]interface{}{"access": "logs/xray-access.log", "error": "", "loglevel": "info"}, "file"},
		{"dev stderr", map[string]interface{}{"access": "none", "error": "/dev/stderr", "loglevel": "info"}, "file"},
		{"off loglevel", map[string]interface{}{"loglevel": "none"}, "off"},
		{"off both none", map[string]interface{}{"access": "none", "error": "none", "loglevel": "info"}, "off"},
		{"file", map[string]interface{}{"access": "logs/access.log", "error": "logs/error.log", "loglevel": "info"}, "file"},
	}
	for _, c := range cases {
		if got := inferXrayLogMode(c.log); got != c.want {
			t.Errorf("%s: inferXrayLogMode = %q, want %q", c.name, got, c.want)
		}
	}
}

// sing-box 的 output=stderr 旧语义是 stdout，同样归一到 file。
func TestInferSingboxLogModeNeverReturnsStdout(t *testing.T) {
	cases := []struct {
		name string
		log  map[string]interface{}
		want string
	}{
		{"stderr", map[string]interface{}{"level": "info", "output": "stderr"}, "file"},
		{"empty", map[string]interface{}{"level": "info"}, "file"},
		{"silent", map[string]interface{}{"level": "silent"}, "off"},
		{"file", map[string]interface{}{"level": "info", "output": "logs/singbox.log"}, "file"},
	}
	for _, c := range cases {
		if got := inferSingboxLogMode(c.log); got != c.want {
			t.Errorf("%s: inferSingboxLogMode = %q, want %q", c.name, got, c.want)
		}
	}
}
