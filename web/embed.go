// Package web embeds the built web interface into the binary.
package web

import (
	"embed"
	"io/fs"
)

// The dist directory is produced by "npm run build".
//
//go:embed all:dist
var dist embed.FS

// FS returns the content of the dist directory.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
