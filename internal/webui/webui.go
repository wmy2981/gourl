// Package webui embeds the built frontend SPA (copied from frontend/dist to
// web/dist by the build script) into the binary, so the admin console ships
// as a single artifact.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

//go:embed icon.svg
var defaultIcon []byte

// Dist returns the embedded frontend filesystem rooted at the dist contents.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// The embed path is static; failure is a build-time error.
		panic(err)
	}
	return sub
}

// DefaultIcon returns the built-in brand icon (SVG bytes).
func DefaultIcon() []byte { return defaultIcon }
