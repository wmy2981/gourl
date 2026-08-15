// Package version holds the application version.
//
// The version defaults to "dev" and is overridden at build time from the
// root VERSION file, e.g.:
//
//	go build -ldflags "-X github.com/wmy2981/gourl/internal/version.Version=0.1.0" ./cmd/gourl
package version

// Version is the application version, injected via -ldflags at build time.
var Version = "dev"
