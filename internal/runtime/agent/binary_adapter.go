package agent

import (
	"context"

	binariesSvc "corefusion/internal/service/binaries"
)

// BinaryInstallerAdapter 包装 binaries.Installer 使其实现 BinaryInstaller 接口。
type BinaryInstallerAdapter struct {
	installer *binariesSvc.Installer
}

// NewBinaryInstallerAdapter 创建适配器。
func NewBinaryInstallerAdapter(installer *binariesSvc.Installer) *BinaryInstallerAdapter {
	return &BinaryInstallerAdapter{installer: installer}
}

func (a *BinaryInstallerAdapter) Install(ctx context.Context, coreType, version, downloadURL, assetName string) (*BinaryInstallResult, error) {
	info, err := a.installer.Install(ctx, coreType, version, downloadURL, assetName)
	if err != nil {
		return nil, err
	}
	return &BinaryInstallResult{
		CoreType:   info.CoreType,
		Version:    info.Version,
		BinaryPath: info.BinaryPath,
		Installed:  info.Installed,
	}, nil
}

func (a *BinaryInstallerAdapter) ListInstalled() []BinaryInstallResult {
	infos := a.installer.ListInstalled()
	results := make([]BinaryInstallResult, len(infos))
	for i, info := range infos {
		results[i] = BinaryInstallResult{
			CoreType:   info.CoreType,
			Version:    info.Version,
			BinaryPath: info.BinaryPath,
			Installed:  info.Installed,
		}
	}
	return results
}

// ReleaseInfo contains GitHub release information.
type ReleaseInfo struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}

func (a *BinaryInstallerAdapter) ListReleases(ctx context.Context, coreType string) ([]ReleaseInfo, error) {
	releases, err := a.installer.ListReleases(ctx, coreType)
	if err != nil {
		return nil, err
	}
	results := make([]ReleaseInfo, len(releases))
	for i, r := range releases {
		results[i] = ReleaseInfo{
			Version:    r.Version,
			Prerelease: r.Prerelease,
		}
	}
	return results, nil
}
