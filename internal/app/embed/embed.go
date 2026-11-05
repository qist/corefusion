package embed

import (
	"embed"
)

//go:embed all:dist
var adminFS embed.FS

// AdminFSBytes returns the raw embed.FS for custom usage.
func AdminFSBytes() embed.FS {
	return adminFS
}
