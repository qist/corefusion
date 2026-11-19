package georule

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// builtinSyncCron 是内置规则源的默认同步周期：每 24 小时一次（凌晨 4 点）。
// 与线上面板里手工建的规则源保持一致；用户仍可改成更短周期或关闭。
const builtinSyncCron = "0 4 * * *"

// legacyBuiltinSourceName 是旧版本「把所有地区规则塞进一个源」的名字。
//
// 已废弃（2026-11-18）：那种写法既不是仓库模式也不是标准直连源，规则页里
// 和其它仓库不一致。现在改成标准写法：
//   - GitHub release 仓库（如 chocolate4u/Iran-v2ray-rules）→ 一个仓库源 + 选中的文件；
//   - raw 单文件（如 sing-geosite 的 geosite-cn.srs）→ 每个 URL 一个直连源。
//
// 启动时会把旧源里的文件迁到对应的标准源，再删掉这条旧源（不删磁盘文件）。
const legacyBuiltinSourceName = "内置路由规则（IR/RU/CN/广告）"

// defaultRouteRuleFiles 是核心默认路由直接引用的规则文件：必须在播种后
// 推送到所有节点，否则 sing-box 找不到本地 rule_set 会启动失败。
var defaultRouteRuleFiles = []string{"geosite-category-ads-all.srs"}

// builtinFileSpec 描述内置规则源里的一个文件。
type builtinFileSpec struct {
	// FileName 是落盘文件名（路由用 ext:/rule_set 引用它）。
	FileName string
	// Asset 是仓库 release 资产名；为空表示与 FileName 相同。
	// 伊朗/俄罗斯仓库自己发的文件都叫 geoip.dat / geosite.dat，和主规则
	// （Loyalsoldier）重名，所以落盘改名成 *_IR.dat / *_RU.dat。
	Asset string
	// URL 仅直连源使用。
	URL string
	// Region 为空表示与地区无关（广告等），否则用于按需确保（cn/ru/ir）。
	Region string
	// Core 为空表示两个核心共用，否则只给指定核心用。
	Core string
}

// builtinSourceSpec 描述一个内置规则源（标准写法：仓库模式或直连模式）。
type builtinSourceSpec struct {
	Name    string
	RepoURL string // 仓库模式为 "owner/repo"；直连模式为完整 URL
	Files   []builtinFileSpec
}

// directBuiltinSource 构造一个「一个 URL 一个源」的直连源（与线上规则库一致）。
func directBuiltinSource(url, region, core string) builtinSourceSpec {
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return builtinSourceSpec{
		Name:    name,
		RepoURL: url,
		Files: []builtinFileSpec{
			{FileName: filepath.Base(url), URL: url, Region: region, Core: core},
		},
	}
}

// builtinSources 是面板默认提供的规则源，全部使用标准写法：
//   - xray 读 v2ray 格式 .dat：GitHub release 仓库（伊朗 / 俄罗斯）+ 主规则
//     （Loyalsoldier，由 SeedDefaults 播种）；
//   - sing-box 读 srs：每个 raw URL 一个直连源（SagerNet / Chocolate4U）。
//
// 广告规则只需要一份 srs（sing-box 默认路由用），xray 用主 geosite.dat 的
// category-ads-all。
var builtinSources = []builtinSourceSpec{
	{
		// 与 Loyalsoldier/v2ray-rules-dat 同样的仓库写法：勾选 release 里的
		// geoip.dat / geosite.dat 两个文件。
		Name:    "chocolate4u/Iran-v2ray-rules",
		RepoURL: "chocolate4u/Iran-v2ray-rules",
		Files: []builtinFileSpec{
			{FileName: "geoip_IR.dat", Asset: "geoip.dat", Region: "ir", Core: "xray"},
			{FileName: "geosite_IR.dat", Asset: "geosite.dat", Region: "ir", Core: "xray"},
		},
	},
	{
		Name:    "runetfreedom/russia-v2ray-rules-dat",
		RepoURL: "runetfreedom/russia-v2ray-rules-dat",
		Files: []builtinFileSpec{
			{FileName: "geoip_RU.dat", Asset: "geoip.dat", Region: "ru", Core: "xray"},
			{FileName: "geosite_RU.dat", Asset: "geosite.dat", Region: "ru", Core: "xray"},
		},
	},
	// xray CN 用主 geosite.dat / geoip.dat（Loyalsoldier），无需额外源。
	// sing-box：中国
	directBuiltinSource("https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", "cn", "singbox"),
	directBuiltinSource("https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", "cn", "singbox"),
	// sing-box：广告
	directBuiltinSource("https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs", "", "singbox"),
	// sing-box：伊朗
	directBuiltinSource("https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geosite-ir.srs", "ir", "singbox"),
	directBuiltinSource("https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geoip-ir.srs", "ir", "singbox"),
	// sing-box：俄罗斯（官方只有 dat；这两个 srs 是生态里可直接下载的等价集合）
	directBuiltinSource("https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ru.srs", "ru", "singbox"),
	directBuiltinSource("https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-ru.srs", "ru", "singbox"),
}

// builtinSeedMarker 记录「内置规则源已经播种过」。
//
// 为什么要一次性标记：旧实现每次启动都会重新补建内置文件（生产实测重启后
// 「added 8 rule files」），用户删掉的 geosite_IR.dat / 地区 srs 会被复活并
// 重新下载 —— 看起来像「定时开关没关掉」。播种只需要一次，之后以内置文件
// 为准的删除是用户明确的意图，不再复活；需要时可在规则页点「恢复默认」。
const builtinSeedMarker = "migration.georule.builtin.seeded"

// EnsureBuiltinFiles 确保内置规则源与文件在库里按标准写法存在，并异步下载。
//
// 幂等：
//   - 源按 RepoURL 判重（已有同 URL 的源就复用，不会重复建）；
//   - 文件按 FileName 判重（旧内置源 / 用户自建的同名文件会被迁到标准源下）；
//   - 旧版聚合源清空后删除，磁盘文件保留；
//   - **只播种一次**（见 builtinSeedMarker）：之后用户删除的文件不再复活。
func (s *Service) EnsureBuiltinFiles(ctx context.Context) {
	if s.settingSvc != nil {
		if v, err := s.settingSvc.GetByKey(ctx, builtinSeedMarker); err == nil && strings.TrimSpace(v) != "" {
			return // 已播种过：尊重用户删除，避免每次启动重新下载
		}
	}
	created, retry := s.ensureBuiltinSourcesInDB(ctx)
	if s.settingSvc != nil {
		if err := s.settingSvc.Set(ctx, builtinSeedMarker, time.Now().Format(time.RFC3339)); err != nil {
			log.Printf("[georule] ensure builtin: write seed marker failed: %v", err)
		}
	}
	if len(created) == 0 && len(retry) == 0 {
		return
	}
	if len(created) > 0 {
		log.Printf("[georule] ensure builtin: added %d rule files across %d sources", len(created), len(builtinSources))
	}

	go func(items, retries []*entity.GeoRuleFile) {
		bg := context.Background()
		for _, r := range retries {
			if _, err := s.downloadLocal(bg, r); err != nil {
				log.Printf("[georule] ensure builtin: retry download %s failed: %v", r.FileName, err)
			}
		}
		for _, r := range items {
			if _, err := s.downloadLocal(bg, r); err != nil {
				log.Printf("[georule] ensure builtin: download %s failed: %v", r.FileName, err)
			}
		}
		// 只推默认路由会引用的文件（sing-box 默认拦截广告要用本地 srs）；
		// 地区文件由「路由页导入」时按需推送到目标节点。
		if err := s.SyncFilesToAllNodes(bg, defaultRouteRuleFiles); err != nil {
			log.Printf("[georule] ensure builtin: push default rule files failed: %v", err)
		}
	}(created, retry)
}

// ensureBuiltinSourcesInDB 只做「源 + 文件记录」的幂等补齐，不触发下载。
// 返回本次新建的文件，以及需要重试下载的默认文件（供调用方异步处理）。
func (s *Service) ensureBuiltinSourcesInDB(ctx context.Context) (created, retry []*entity.GeoRuleFile) {
	if ctx == nil {
		ctx = context.Background()
	}
	os.MkdirAll(filepath.Join(s.dataDir, "rules"), 0o750)

	files, err := s.fileRepo.ListByServer(ctx, 0)
	if err != nil {
		log.Printf("[georule] ensure builtin: list files failed: %v", err)
		return nil, nil
	}
	byFile := make(map[string]*entity.GeoRuleFile, len(files))
	for i := range files {
		byFile[files[i].FileName] = &files[i]
	}

	sources, err := s.sourceRepo.ListByServer(ctx, 0)
	if err != nil {
		log.Printf("[georule] ensure builtin: list sources failed: %v", err)
		return nil, nil
	}
	byRepo := make(map[string]*entity.RuleSource, len(sources))
	for i := range sources {
		byRepo[normalizeRepoKey(sources[i].RepoURL)] = &sources[i]
	}

	created = make([]*entity.GeoRuleFile, 0, len(builtinSources))
	for _, spec := range builtinSources {
		src := s.ensureBuiltinSource(ctx, spec, byRepo)
		if src == nil {
			continue
		}
		for _, f := range spec.Files {
			url := builtinFileURL(spec, f)
			asset := builtinAssetName(f)
			if row := byFile[f.FileName]; row != nil {
				// 已有同文件（旧内置源 / 用户自建）：归到标准源并刷新地址。
				changed := false
				if row.SourceID != src.ID {
					row.SourceID = src.ID
					changed = true
				}
				if row.SourceAsset != asset {
					// 补上「仓库原始文件名」，前端据此显示「重命名」。
					row.SourceAsset = asset
					changed = true
				}
				if row.Name != asset {
					// 历史记录的 Name 是落盘名，统一成仓库资产名（判重用）。
					row.Name = asset
					changed = true
				}
				if row.DownloadURL != url {
					row.DownloadURL = url
					changed = true
				}
				// 注意：不再强制把 AutoSync 改回 true。
				// 用户关掉单个文件的「自动同步」是明确意图，启动播种不能覆盖。
				if changed {
					if err := s.fileRepo.Update(ctx, row); err != nil {
						log.Printf("[georule] ensure builtin: migrate file %s failed: %v", row.FileName, err)
					}
				}
				continue
			}
			row := &entity.GeoRuleFile{
				ServerID:    0,
				SourceID:    src.ID,
				Name:        asset,
				FileName:    f.FileName,
				SourceAsset: asset,
				DownloadURL: url,
				AutoSync:    true,
				Status:      "pending",
			}
			if err := s.fileRepo.Create(ctx, row); err != nil {
				log.Printf("[georule] ensure builtin: create %s failed: %v", f.FileName, err)
				continue
			}
			byFile[f.FileName] = row
			created = append(created, row)
		}
	}

	s.cleanupLegacyBuiltinSource(ctx)

	// 默认路由引用的文件必须可用（上次下载中断/失败要重试），
	// 顺带补一次推送：节点可能在上次推送时离线。
	retry = make([]*entity.GeoRuleFile, 0, len(defaultRouteRuleFiles))
	for _, name := range defaultRouteRuleFiles {
		row := byFile[name]
		if row == nil || row.Status == "available" {
			continue
		}
		retry = append(retry, row)
	}
	return created, retry
}

// ensureBuiltinSource 按 RepoURL 找到或创建标准规则源。
// 新建的源默认启用 + 24h 同步（用户之后可以自行关闭/改周期）；
// 已存在的源不强制改 enabled，只补空的同步周期。
func (s *Service) ensureBuiltinSource(ctx context.Context, spec builtinSourceSpec, byRepo map[string]*entity.RuleSource) *entity.RuleSource {
	key := normalizeRepoKey(spec.RepoURL)
	if src := byRepo[key]; src != nil {
		changed := false
		// 直连源的历史名字可能带着扩展名（geosite-cn.srs），统一成文件名主干。
		if isDirectURL(src.RepoURL) && src.Name != spec.Name {
			src.Name = spec.Name
			changed = true
		}
		if strings.TrimSpace(src.SyncCron) == "" {
			src.SyncCron = builtinSyncCron
			changed = true
		}
		if changed {
			if err := s.sourceRepo.Update(ctx, src); err != nil {
				log.Printf("[georule] ensure builtin: update source %s failed: %v", spec.Name, err)
			}
		}
		return src
	}

	src := &entity.RuleSource{
		ServerID: 0,
		Name:     spec.Name,
		RepoURL:  spec.RepoURL,
		Enabled:  true,
		SyncCron: builtinSyncCron,
		Status:   "idle",
	}
	if err := s.sourceRepo.Create(ctx, src); err != nil {
		log.Printf("[georule] ensure builtin: create source %s failed: %v", spec.Name, err)
		return nil
	}
	byRepo[key] = src
	return src
}

// builtinAssetName 返回仓库里的原始资产名（默认与落盘名相同）。
func builtinAssetName(f builtinFileSpec) string {
	asset := strings.TrimSpace(f.Asset)
	if asset == "" {
		asset = f.FileName
	}
	return asset
}

// builtinFileURL 返回文件的下载地址。
// 仓库模式统一用 releases/latest/download/<asset>：永远指向最新版，
// 定时同步直接重下即可，不依赖 GitHub API（API 有速率限制）。
func builtinFileURL(spec builtinSourceSpec, f builtinFileSpec) string {
	if isDirectURL(spec.RepoURL) {
		return spec.RepoURL
	}
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repoSlug(spec.RepoURL), builtinAssetName(f))
}

// normalizeRepoKey 把 "https://github.com/Owner/Repo.git" 归一化成
// "owner/repo"，用于仓库源判重（GitHub 仓库名大小写不敏感）。
func normalizeRepoKey(repoURL string) string {
	return strings.TrimSuffix(strings.ToLower(repoSlug(repoURL)), ".git")
}

// repoSlug 去掉 GitHub 主机前缀与 .git 后缀，保留原始大小写（拼下载地址用）。
func repoSlug(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	lower := strings.ToLower(s)
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
		if strings.HasPrefix(lower, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	s = strings.TrimSuffix(s, "/")
	return strings.TrimSuffix(s, ".git")
}

// cleanupLegacyBuiltinSource 删除旧版聚合源（文件已经迁到标准源），
// 若它下面还剩不认识的用户文件则保留，避免误删数据。
func (s *Service) cleanupLegacyBuiltinSource(ctx context.Context) {
	sources, err := s.sourceRepo.ListByServer(ctx, 0)
	if err != nil {
		return
	}
	for _, src := range sources {
		if src.Name != legacyBuiltinSourceName {
			continue
		}
		files, err := s.fileRepo.ListBySource(ctx, src.ID)
		if err != nil {
			continue
		}
		if len(files) > 0 {
			log.Printf("[georule] ensure builtin: legacy source %d still has %d unknown files, keeping", src.ID, len(files))
			continue
		}
		if err := s.sourceRepo.Delete(ctx, src.ID); err != nil {
			log.Printf("[georule] ensure builtin: remove legacy source %d failed: %v", src.ID, err)
			continue
		}
		log.Printf("[georule] ensure builtin: removed legacy aggregated source %d (files migrated)", src.ID)
	}
}

// regionFiles 返回某核心某地区需要的规则文件名。
func regionFiles(coreType, region string) []string {
	out := make([]string, 0, 2)
	for _, spec := range builtinSources {
		for _, f := range spec.Files {
			if f.Core != "" && f.Core != coreType {
				continue
			}
			if f.Region != region {
				continue
			}
			out = append(out, f.FileName)
		}
	}
	return out
}

// EnsureRegionFiles 确保地区规则文件已下载到面板并推送到目标节点。
// 返回错误表示文件仍不可用（调用方不应写入引用它的路由规则，否则核心起不来）。
func (s *Service) EnsureRegionFiles(ctx context.Context, serverID int64, coreType, region string) error {
	names := regionFiles(coreType, region)
	if len(names) == 0 {
		return nil
	}

	// xray 的 CN 用主 geosite.dat / geoip.dat（已由 SeedDefaults 播种）。
	if coreType == "xray" && region == "cn" {
		names = []string{"geosite.dat", "geoip.dat"}
	}

	files, err := s.fileRepo.ListByServer(ctx, 0)
	if err != nil {
		return err
	}
	byName := make(map[string]*entity.GeoRuleFile, len(files))
	for i := range files {
		byName[files[i].FileName] = &files[i]
	}

	for _, name := range names {
		f, ok := byName[name]
		if !ok {
			return fmt.Errorf("规则文件缺失：%s（请先在规则库同步）", name)
		}
		diskPath := filepath.Join(s.dataDir, "rules", name)
		if f.Status == "available" {
			if fi, statErr := os.Stat(diskPath); statErr == nil && fi.Size() > 0 {
				continue
			}
		}
		if _, err := s.downloadLocal(ctx, f); err != nil {
			return err
		}
	}

	// 推送到目标节点（本地节点直接跳过）。
	if err := s.SyncToNode(ctx, serverID); err != nil {
		return err
	}
	return nil
}

// BuiltinRuleFiles 返回内置规则文件清单（只读，供测试/排查）。
func BuiltinRuleFiles() []string {
	out := make([]string, 0, len(builtinSources))
	for _, spec := range builtinSources {
		for _, f := range spec.Files {
			out = append(out, f.FileName)
		}
	}
	return out
}
