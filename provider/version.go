package provider

// Version is initialized by the Go linker to contain the semver of this build.
//
// Example: go build -ldflags "-X github.com/highfiveghost/pulumi-osano/provider.Version=v0.0.1"
var Version string

// Name controls how this provider is referenced in package names and elsewhere.
const Name string = "osano"
