// Package constants provides shared build-time constants for CoreFusion binaries.
//
// Version is the single source of truth for the program version, used by both
// the Panel (cmd/corefusion) and the Agent (cmd/agent).
//
// Resolution order:
//  1. -ldflags override: go build -ldflags "-X corefusion/internal/constants.Version=v2.0" .
//  2. Embedded version file (internal/constants/version)
package constants

import (
	_ "embed"
	"strings"
)

//go:embed version
var versionFile string

// Version holds the program version. It can be overridden at build time via
// -ldflags:
//
//	go build -ldflags "-X corefusion/internal/constants.Version=v2.0" ./cmd/corefusion
//	go build -ldflags "-X corefusion/internal/constants.Version=v2.0" ./cmd/agent
var Version = ""

func init() {
	if Version == "" {
		Version = strings.TrimSpace(versionFile)
	}
}
