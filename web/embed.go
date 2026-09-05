// Package web embeds the Malice single-page UI so the binary is self-contained.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

// FS returns the embedded UI rooted at the static/ directory.
func FS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
