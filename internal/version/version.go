// Package version exposes build information injected at link time.
package version

// These values are overridden at build time with -ldflags "-X ...".
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
