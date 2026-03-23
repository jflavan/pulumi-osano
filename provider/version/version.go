// Package version exposes the provider version set at build time.
//
//nolint:goheader // Source-file header normalization is still in progress during alpha.
package version

// Version is set at build time via -ldflags to the provider's semver string.
var Version = "0.0.0"
