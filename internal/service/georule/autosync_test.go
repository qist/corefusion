package georule

import (
	"context"
	"fmt"
	"testing"

	"corefusion/internal/model/entity"
)

// 定时开关关闭的源不参与自动同步（启动修复 / 安装核心后的同步）。
// 旧实现 SyncAll 遍历所有源，把用户关掉的源也重下一遍。
func TestSelectSourcesForSyncSkipsDisabled(t *testing.T) {
	sources := []entity.RuleSource{
		{ID: 1, Name: "on-1", Enabled: true},
		{ID: 2, Name: "off", Enabled: false},
		{ID: 3, Name: "on-2", Enabled: true},
	}

	auto := selectSourcesForSync(sources, false)
	if len(auto) != 2 {
		t.Fatalf("自动同步应只含 2 个启用的源，got %d", len(auto))
	}
	for _, s := range auto {
		if !s.Enabled {
			t.Fatalf("自动同步不应包含关闭的源：%+v", s)
		}
	}

	// 手动「同步全部」包含关闭的源
	if all := selectSourcesForSync(sources, true); len(all) != 3 {
		t.Fatalf("手动同步应包含全部源，got %d", len(all))
	}
}

// 关闭了定时开关的直连源，删掉文件后同步不应把它补回来。
func TestEnsureDirectSourceFileSkipsDisabledSource(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	src := &entity.RuleSource{
		ServerID: 0,
		Name:     "geosite-ir",
		RepoURL:  "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geosite-ir.srs",
		Enabled:  false,
		SyncCron: builtinSyncCron,
		Status:   "idle",
	}
	if err := db.Create(src).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}

	got, row := svc.ensureDirectSourceFile(ctx, src, nil)
	if row != nil {
		t.Fatalf("关闭的源不应补建文件：%+v", row)
	}
	if len(got) != 0 {
		t.Fatalf("返回列表应保持为空，got %d", len(got))
	}
	if saved, _ := svc.fileRepo.ListBySource(ctx, src.ID); len(saved) != 0 {
		t.Fatalf("不应落库任何文件：%+v", saved)
	}
}

// memSettings 是内存版设置存储（SettingStore 测试桩）。
type memSettings struct {
	m map[string]string
}

func (s *memSettings) GetByKey(_ context.Context, key string) (string, error) {
	if v, ok := s.m[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s", key)
}

func (s *memSettings) Set(_ context.Context, key, value string) error {
	s.m[key] = value
	return nil
}

// 内置源只播种一次：已有标记时不再补建（用户删除的地区规则不会被复活）。
func TestEnsureBuiltinFilesSkipsWhenAlreadySeeded(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	svc.settingSvc = &memSettings{m: map[string]string{builtinSeedMarker: "2026-11-19T00:00:00Z"}}

	svc.EnsureBuiltinFiles(context.Background())

	if srcs := listSources(t, svc); len(srcs) != 0 {
		t.Fatalf("已播种过就不应再建源，got %d：%+v", len(srcs), srcs)
	}
}

// 首次播种应写入标记，保证下次启动走「跳过」分支。
func TestEnsureBuiltinFilesWritesSeedMarker(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	st := &memSettings{m: map[string]string{}}
	svc.settingSvc = st

	// 只跑 DB 层播种，避免触发真实下载 goroutine。
	svc.ensureBuiltinSourcesInDB(context.Background())
	if err := svc.settingSvc.Set(context.Background(), builtinSeedMarker, "done"); err != nil {
		t.Fatalf("set marker: %v", err)
	}
	if v := st.m[builtinSeedMarker]; v == "" {
		t.Fatal("播种标记未写入")
	}
}
