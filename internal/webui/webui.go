// Package webui embeds the built frontend SPA (copied from frontend/dist to
// web/dist by the build script) into the binary, so the admin console ships
// as a single artifact.
package webui

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"

	"github.com/wmy2981/gourl/internal/version"
)

//go:embed all:dist
var dist embed.FS

//go:embed all:swaggerui
//go:embed openapi.yaml
//go:embed icon.svg
var assets embed.FS

//go:embed all:locales
var locales embed.FS

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

// PageLocale returns the "page" strings for lang ("en" or "zh") from the
// embedded locale files. The build script copies them from
// frontend/src/locales — the single source of user-facing copy — so the
// backend-rendered pages speak the same language as the SPA.
func PageLocale(lang string) map[string]string {
	data, err := locales.ReadFile("locales/" + lang + ".json")
	if err != nil {
		// The embed pattern is static and the build script always copies
		// both files; failure is a build-time error.
		panic(err)
	}
	var doc struct {
		Page map[string]string `json:"page"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		// The SPA build validates the same files; a malformed locale is
		// caught before this binary is ever built.
		panic(err)
	}
	return doc.Page
}

// Docs returns the embedded swagger-ui assets filesystem (index.html,
// swagger-ui-bundle.js, ...).
func Docs() fs.FS {
	sub, err := fs.Sub(assets, "swaggerui")
	if err != nil {
		// The embed path is static; failure is a build-time error.
		panic(err)
	}
	return sub
}

// OpenAPISpec returns the embedded OpenAPI 3.0 specification (YAML bytes),
// with the info.version placeholder replaced by the build version so the
// Swagger UI matches /api/v1/health and the footer.
func OpenAPISpec() []byte {
	data, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		// The embed path is static; failure is a build-time error.
		panic(err)
	}
	return bytes.ReplaceAll(data, []byte(VersionPlaceholder), []byte(version.Version))
}

// VersionPlaceholder marks info.version in openapi.yaml; the server replaces
// it with the build version on every request and the Pages workflow with the
// VERSION file at deploy time.
const VersionPlaceholder = "__GOURL_VERSION__"
