package embed

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var adminFS embed.FS

// AdminFS returns the embedded admin frontend filesystem.
func AdminFS() http.FileSystem {
	fsys, err := fs.Sub(adminFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	return http.FS(fsys)
}

// AdminFSBytes returns the raw embed.FS for custom usage.
func AdminFSBytes() embed.FS {
	return adminFS
}
