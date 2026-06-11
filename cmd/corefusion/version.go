package main

import (
	_ "embed"
	"strings"
)

//go:embed version
var versionFile string

// Version is the program version, read from the embedded version file.
// Can be overridden at build time via -ldflags:
//
//	go build -ldflags "-X 'main.Version=v2.0'" .
var Version = ""

func init() {
	if Version == "" {
		Version = strings.TrimSpace(versionFile)
	}
}
