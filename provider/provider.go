package provider

import (
    "fmt"

    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
    "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Provider creates a new instance of the Osano provider with all supported resources.
func Provider() p.Provider {
    prov, err := infer.NewProviderBuilder().
        WithDisplayName("Osano").
        WithDescription("Pulumi provider for managing Osano via the Osano APIs (Customer REST API and Unified Consent Core API). ").
        WithHomepage("https://developers.osano.com/").
        WithRepository("https://github.com/jflavan/pulumi-osano").
        WithNamespace(Name).
        WithConfig(infer.Config(&Config{})).
        WithResources(
            infer.Resource(&CookieConsentConfig{}),
            infer.Resource(&CookieConsentRule{}),
        ).
        WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
            "provider": "index",
        }).
        Build()
    if err != nil {
        panic(fmt.Errorf("unable to build provider: %w", err))
    }
    return prov
}
