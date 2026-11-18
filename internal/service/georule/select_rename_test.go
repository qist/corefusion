package georule

import (
	"context"
	"encoding/json"
	"testing"

	"corefusion/internal/model/entity"
)

func TestSelectFileInputUnmarshal(t *testing.T) {
	// 老写法：纯字符串数组
	var legacy []SelectFileInput
	if err := json.Unmarshal([]byte(`["geoip.dat","geosite.dat"]`), &legacy); err != nil {
		t.Fatalf("legacy unmarshal: %v", err)
	}
	if len(legacy) != 2 || legacy[0].Name != "geoip.dat" || legacy[0].Rename != "" {
		t.Fatalf("legacy decode = %+v", legacy)
	}

	// 新写法：带重命名
	var withRename []SelectFileInput
	if err := json.Unmarshal([]byte(`[{"name":"geoip.dat","rename":"geoip_IR.dat"}]`), &withRename); err != nil {
		t.Fatalf("rename unmarshal: %v", err)
	}
	if len(withRename) != 1 || withRename[0].Name != "geoip.dat" || withRename[0].Rename != "geoip_IR.dat" {
		t.Fatalf("rename decode = %+v", withRename)
	}
}

func TestValidateRuleFileName(t *testing.T) {
	for _, ok := range []string{"geoip_IR.dat", "geosite-cn.srs", "my_rules.json"} {
		if err := ValidateRuleFileName(ok); err != nil {
			t.Errorf("%q 应合法: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "..", "../etc/passwd", "a/b.dat", `a\b.dat`, "x..dat"} {
		if err := ValidateRuleFileName(bad); err == nil {
			t.Errorf("%q 应被拒绝", bad)
		}
	}
}

// 落盘名全局唯一：不能占用别的源已经在用的名字（否则会覆盖主规则文件）。
func TestEnsureFileNameFree(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	src1 := &entity.RuleSource{ServerID: 0, Name: "s1", RepoURL: "owner/repo1", Status: "idle"}
	src2 := &entity.RuleSource{ServerID: 0, Name: "s2", RepoURL: "owner/repo2", Status: "idle"}
	if err := db.Create(src1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(src2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&entity.GeoRuleFile{
		ServerID: 0, SourceID: src1.ID, Name: "geoip.dat", FileName: "geoip.dat",
		DownloadURL: "https://example.com/geoip.dat", Status: "available",
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ensureFileNameFree(ctx, src2.ID, "geoip.dat"); err == nil {
		t.Errorf("跨源同名应被拒绝")
	}
	if err := svc.ensureFileNameFree(ctx, src2.ID, "geoip_IR.dat"); err != nil {
		t.Errorf("未占用名字应可用: %v", err)
	}
	if err := svc.ensureFileNameFree(ctx, src1.ID, "geoip.dat"); err == nil {
		t.Errorf("同源同名也应提示换名（避免两个文件写同一路径）")
	}
}

// 内置的伊朗/俄罗斯文件必须带「仓库原始资产名」，前端才能标注重命名。
func TestBuiltinFilesExposeSourceAsset(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	svc.ensureBuiltinSourcesInDB(context.Background())

	files, err := svc.fileRepo.ListByServer(context.Background(), 0)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	byName := map[string]entity.GeoRuleFile{}
	for _, f := range files {
		byName[f.FileName] = f
	}

	cases := []struct{ file, asset string }{
		{"geoip_IR.dat", "geoip.dat"},
		{"geosite_IR.dat", "geosite.dat"},
		{"geoip_RU.dat", "geoip.dat"},
		{"geosite_RU.dat", "geosite.dat"},
		{"geosite-cn.srs", "geosite-cn.srs"},
	}
	for _, c := range cases {
		f, ok := byName[c.file]
		if !ok {
			t.Fatalf("缺少文件 %s", c.file)
		}
		if f.SourceAsset != c.asset {
			t.Errorf("%s source_asset = %q, want %q", c.file, f.SourceAsset, c.asset)
		}
	}
}
