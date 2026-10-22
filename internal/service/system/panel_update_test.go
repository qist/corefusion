package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// writeFakeELF 写入以 ELF magic 开头的假二进制。
func writeFakeELF(t *testing.T, path, payload string) {
	t.Helper()
	data := append([]byte{0x7f, 'E', 'L', 'F'}, []byte(payload)...)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newPanelUpdateSvcForTest(t *testing.T) (*PanelUpdateService, *gorm.DB, string) {
	t.Helper()
	dir := t.TempDir()
	g := openSQLiteForTest(t, filepath.Join(dir, "panel-update.db"))
	if err := g.AutoMigrate(&entity.PanelUpdateJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	exe := filepath.Join(dir, "corefusion")
	writeFakeELF(t, exe, "current")
	svc := NewPanelUpdateService(g, nil, dir, "v0.12.298")
	svc.executableFn = func() (string, error) { return exe, nil }
	svc.SetRestartFunc(func() {})
	return svc, g, exe
}

func TestNormalizeSHA256(t *testing.T) {
	valid := strings.Repeat("a", 64)
	cases := map[string]string{
		valid:             valid,
		"sha256:" + valid: valid,
		"  SHA256:" + strings.ToUpper(valid) + "  ": valid,
		strings.Repeat("a", 63):                     "",
		strings.Repeat("z", 64):                     "",
		"":                                          "",
	}
	for in, want := range cases {
		if got := normalizeSHA256(in); got != want {
			t.Errorf("normalizeSHA256(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerifyELF(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	writeFakeELF(t, good, "x")
	if err := verifyELF(good); err != nil {
		t.Errorf("valid ELF should pass: %v", err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("<html>404</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyELF(bad); err == nil {
		t.Error("HTML file must be rejected")
	}
}

func TestPanelRollbackRestoresBackup(t *testing.T) {
	svc, _, exe := newPanelUpdateSvcForTest(t)
	bak := exe + ".bak"
	writeFakeELF(t, bak, "previous")
	if err := os.WriteFile(bak+".json", []byte(`{"version":"v0.12.100","created_at":"2026-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	info := svc.RollbackInfo()
	if !info.Available || info.Version != "v0.12.100" {
		t.Fatalf("rollback info = %+v", info)
	}

	job, err := svc.StartRollback(context.Background())
	if err != nil {
		t.Fatalf("start rollback: %v", err)
	}
	if job.ToVersion != "v0.12.100" {
		t.Errorf("job.ToVersion = %q, want v0.12.100", job.ToVersion)
	}
	waitJobStatus(t, svc, job.ID, PanelUpdateStatusSuccess)

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "previous") {
		t.Errorf("binary was not restored from backup: %q", data)
	}
}

func TestPanelRollbackWithoutBackup(t *testing.T) {
	svc, _, _ := newPanelUpdateSvcForTest(t)
	if _, err := svc.StartRollback(context.Background()); err != ErrPanelRollbackUnavailable {
		t.Fatalf("err = %v, want ErrPanelRollbackUnavailable", err)
	}
}

func TestPanelUpdateRejectsConcurrentJob(t *testing.T) {
	svc, g, _ := newPanelUpdateSvcForTest(t)
	job := &entity.PanelUpdateJob{Action: PanelUpdateActionUpdate, Status: PanelUpdateStatusRunning}
	if err := g.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartRollback(context.Background()); err != ErrPanelUpdateInProgress {
		t.Fatalf("err = %v, want ErrPanelUpdateInProgress", err)
	}
}

func TestPanelUpdateJobListOrdering(t *testing.T) {
	svc, g, _ := newPanelUpdateSvcForTest(t)
	for _, st := range []string{PanelUpdateStatusFailed, PanelUpdateStatusSuccess} {
		if err := g.Create(&entity.PanelUpdateJob{Action: PanelUpdateActionUpdate, Status: st}).Error; err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := svc.ListJobs(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	if items[0].ID < items[1].ID {
		t.Error("jobs should be ordered newest first")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestListReleasesParsesGitHubResponse(t *testing.T) {
	svc, _, _ := newPanelUpdateSvcForTest(t)
	body, _ := json.Marshal([]map[string]any{
		{
			"tag_name":     "v0.13.0-alpha.1",
			"name":         "v0.13.0-alpha.1",
			"prerelease":   true,
			"draft":        false,
			"published_at": "2026-10-22T08:00:00Z",
			"body":         "alpha",
			"assets": []map[string]any{
				{"name": "corefusion-linux-amd64", "size": 1234, "browser_download_url": "https://example.com/a", "digest": "sha256:" + strings.Repeat("b", 64)},
				{"name": "corefusion-linux-amd64.sha256", "size": 100, "browser_download_url": "https://example.com/a.sha256"},
			},
		},
	})
	svc.httpClientFn = func(context.Context) *http.Client {
		return &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     make(http.Header),
			}, nil
		})}
	}

	res, err := svc.ListReleases(context.Background(), 5)
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(res.Releases) != 1 {
		t.Fatalf("releases = %d", len(res.Releases))
	}
	r := res.Releases[0]
	if r.Version != "v0.13.0-alpha.1" || !r.Prerelease {
		t.Errorf("unexpected release: %+v", r)
	}
	if r.Asset == nil || r.Asset.DownloadURL != "https://example.com/a" {
		t.Errorf("asset not matched: %+v", r.Asset)
	}
	if r.ChecksumURL != "https://example.com/a.sha256" {
		t.Errorf("checksum url = %q", r.ChecksumURL)
	}
	if res.LatestVersion != "v0.13.0-alpha.1" || !res.UpdateAvailable {
		t.Errorf("latest=%q update=%v", res.LatestVersion, res.UpdateAvailable)
	}
}

func waitJobStatus(t *testing.T, svc *PanelUpdateService, id int64, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var job entity.PanelUpdateJob
		if err := svc.db.First(&job, id).Error; err == nil && job.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	var job entity.PanelUpdateJob
	_ = svc.db.First(&job, id).Error
	t.Fatalf("job %d status = %q (err=%q), want %q", id, job.Status, job.ErrorMessage, want)
}
