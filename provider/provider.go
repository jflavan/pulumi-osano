package provider

import (
	"fmt"

	provVersion "github.com/jflavan/pulumi-osano/provider/version"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is exported so the gRPC server can announce the plugin version to Pulumi.
var Version = provVersion.Version

// providerVersion is used inside HTTP clients to build user-agent strings.
var providerVersion = provVersion.Version

// Name controls the namespace used by all resources in this provider.
const Name = "osano"

// Provider wires up the Pulumi provider with its config, resources, and functions.
func Provider() p.Provider {
	if Version != "" {
		providerVersion = Version
	}

	prov, err := infer.NewProviderBuilder().
		WithDisplayName("Osano (Unofficial)").
		WithDescription(
			"Unofficial Pulumi provider for managing Osano Unified Consent resources. "+
				"Not affiliated with Pulumi Corporation or Osano, Inc.",
		).
		WithHomepage("https://github.com/jflavan/pulumi-osano").
		WithRepository("https://github.com/jflavan/pulumi-osano").
		WithPluginDownloadURL("github://api.github.com/jflavan/pulumi-osano").
		WithNamespace(Name).
		WithConfig(infer.Config(&Config{})).
		WithResources(
			infer.Resource(&ConsentResource{}),
		).
		WithFunctions(
			infer.Function(&GetUnifiedConsent{}),
			infer.Function(&GetSubject{}),
			infer.Function(&GetConfig{}),
			infer.Function(&GetCollections{}),
			infer.Function(&GetCollection{}),
			infer.Function(&CheckConsent{}),
			infer.Function(&GetConsentProfile{}),
			infer.Function(&SendSubjectCode{}),
			infer.Function(&VerifySubjectCode{}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		WithLanguageMap(map[string]any{
			"csharp": map[string]any{
				"rootNamespace":        "Community.Pulumi",
				"respectSchemaVersion": true,
			},
			"java": map[string]any{
				"basePackage": "io.github.jflavan.pulumi",
				"buildFiles":  "gradle",
			},
			"nodejs": map[string]any{
				"packageName":          "@jflavan/pulumi-osano",
				"packageDescription":   "Pulumi provider for the Osano Unified Consent API.",
				"respectSchemaVersion": true,
			},
			"python": map[string]any{
				"packageName":        "pulumi_osano",
				"packageDescription": "Pulumi provider for the Osano Unified Consent API.",
			},
		}).
		Build()
	if err != nil {
		panic(fmt.Errorf("unable to build provider: %w", err))
	}
	return prov
}
