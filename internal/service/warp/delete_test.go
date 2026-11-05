package warp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"corefusion/internal/model/entity"
)

// withFakeWarpAPI 把包级 base URL 临时指向 httptest 服务。
func withFakeWarpAPI(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := warpAPIBaseURL
	warpAPIBaseURL = srv.URL
	return func() {
		warpAPIBaseURL = old
		srv.Close()
	}
}

// TestDeleteWarpDeviceUsesTunnelSpecificEndpoint 校验两类记录各走自己的 API 版本与请求头。
func TestDeleteWarpDeviceUsesTunnelSpecificEndpoint(t *testing.T) {
	type captured struct {
		method, path, version, auth, ua string
	}
	var got captured
	stop := withFakeWarpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		got = captured{
			method:  r.Method,
			path:    r.URL.Path,
			version: r.Header.Get("CF-Client-Version"),
			auth:    r.Header.Get("Authorization"),
			ua:      r.Header.Get("User-Agent"),
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer stop()

	if err := deleteWarpDevice(&entity.WarpConfig{
		DeviceID: "dev-masque", Token: "tok-1", TunnelType: "masque",
	}); err != nil {
		t.Fatalf("masque delete: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/v0a4471/reg/dev-masque" {
		t.Errorf("masque request = %s %s", got.method, got.path)
	}
	if got.version != masqueClientVersion || got.ua != masqueUserAgent {
		t.Errorf("masque headers version=%q ua=%q", got.version, got.ua)
	}
	if got.auth != "Bearer tok-1" {
		t.Errorf("masque auth = %q", got.auth)
	}

	if err := deleteWarpDevice(&entity.WarpConfig{
		DeviceID: "dev-wg", Token: "tok-2", TunnelType: "wireguard",
	}); err != nil {
		t.Fatalf("wireguard delete: %v", err)
	}
	if got.path != "/v0a2158/reg/dev-wg" {
		t.Errorf("wireguard path = %s", got.path)
	}
	if got.version != "a-6.10-2158" {
		t.Errorf("wireguard version = %q", got.version)
	}
}

// TestDeleteWarpDeviceTreatsNotFoundAsDeleted 幂等：云端已不存在时视为成功。
func TestDeleteWarpDeviceTreatsNotFoundAsDeleted(t *testing.T) {
	stop := withFakeWarpAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer stop()

	if err := deleteWarpDevice(&entity.WarpConfig{DeviceID: "gone", Token: "t", TunnelType: "masque"}); err != nil {
		t.Fatalf("404 should be treated as deleted: %v", err)
	}
}

// TestServiceDeleteLocalOnlyDoesNotCallAPI remote=false 时不应发起任何请求。
func TestServiceDeleteLocalOnlyDoesNotCallAPI(t *testing.T) {
	called := 0
	stop := withFakeWarpAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	})
	defer stop()

	svc, _ := newMasqueTestService(t)
	cfg := masqueWarpConfig()
	cfg.ID = 0
	if err := svc.repo.Create(nil, cfg); err != nil {
		t.Fatalf("create warp cfg: %v", err)
	}

	res, err := svc.Delete(cfg.ID, false)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if called != 0 {
		t.Errorf("remote API called %d times with remote=false", called)
	}
	if res.RemoteAttempted || res.RemoteDeleted {
		t.Errorf("unexpected remote flags: %#v", res)
	}
	if _, err := svc.repo.GetByID(nil, cfg.ID); err == nil {
		t.Error("local record not deleted")
	}
}

// TestServiceDeleteRemoteFailureStillRemovesLocal 云端失败不阻塞本地删除，错误通过结果返回。
func TestServiceDeleteRemoteFailureStillRemovesLocal(t *testing.T) {
	stop := withFakeWarpAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	defer stop()

	svc, _ := newMasqueTestService(t)
	cfg := masqueWarpConfig()
	cfg.ID = 0
	if err := svc.repo.Create(nil, cfg); err != nil {
		t.Fatalf("create warp cfg: %v", err)
	}

	res, err := svc.Delete(cfg.ID, true)
	if err != nil {
		t.Fatalf("delete should succeed locally: %v", err)
	}
	if !res.RemoteAttempted || res.RemoteDeleted {
		t.Errorf("remote flags = %#v", res)
	}
	if res.RemoteError == "" {
		t.Error("expected remote error message")
	}
	if _, err := svc.repo.GetByID(nil, cfg.ID); err == nil {
		t.Error("local record should still be deleted")
	}
}

// TestServiceDeleteRemoteSuccess 云端 204 时标记为已注销。
func TestServiceDeleteRemoteSuccess(t *testing.T) {
	stop := withFakeWarpAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer stop()

	svc, _ := newMasqueTestService(t)
	cfg := masqueWarpConfig()
	cfg.ID = 0
	if err := svc.repo.Create(nil, cfg); err != nil {
		t.Fatalf("create warp cfg: %v", err)
	}

	res, err := svc.Delete(cfg.ID, true)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !res.RemoteAttempted || !res.RemoteDeleted || res.RemoteError != "" {
		t.Errorf("remote flags = %#v", res)
	}
}
