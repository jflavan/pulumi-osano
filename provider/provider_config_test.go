package provider

import (
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func providerConfigURN() resource.URN {
	return resource.NewURN("test-stack", "test-project", "", tokens.Type("pulumi:providers:osano"), "default")
}

// The engine checks and diffs provider config on a provider that has not been configured yet, so
// these tests use an unconfigured server; a configured one would echo its live settings from Check.
func newUnconfiguredProviderServer(t *testing.T) integration.Server {
	t.Helper()
	server, err := integration.NewServer(
		t.Context(), Name, semver.MustParse("1.0.0"), integration.WithProvider(Provider()),
	)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// Provider configuration only selects credentials, endpoints, and timeouts. None of it changes the
// identity of an upstream Cookie Consent config or rule, and Osano cannot delete configs, so a
// provider config change must never make Pulumi replace the resources it manages.
func TestProviderConfigChangesNeverReplaceResources(t *testing.T) {
	server := newUnconfiguredProviderServer(t)
	old := property.NewMap(map[string]property.Value{
		"customerBaseUrl": property.New("https://api.osano.com"),
		"osanoApiKey":     property.New("old-key").WithSecret(true),
		"version":         property.New("0.1.0-alpha.0+dev"),
	})
	cases := map[string]property.Map{
		"rotated API key":         old.Set("osanoApiKey", property.New("new-key").WithSecret(true)),
		"added request timeout":   old.Set("requestTimeoutSeconds", property.New(30.0)),
		"changed customer domain": old.Set("customerBaseUrl", property.New("https://sandbox.example.com")),
		"added Unified Consent key": old.Set(
			"unifiedConsentApiKey", property.New("uc-key").WithSecret(true),
		),
	}
	for name, news := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := server.DiffConfig(p.DiffRequest{Urn: providerConfigURN(), State: old, Inputs: news})
			if err != nil {
				t.Fatal(err)
			}
			if !resp.HasChanges {
				t.Fatal("expected the provider config change to be reported")
			}
			for key, diff := range resp.DetailedDiff {
				if diff.Kind == p.AddReplace || diff.Kind == p.DeleteReplace || diff.Kind == p.UpdateReplace {
					t.Fatalf("provider config key %q must not force replacement, got %v", key, diff.Kind)
				}
			}
		})
	}
}

// pulumi import records the raw stack config as the default provider's inputs, while a program
// run records the checked inputs (every field present, empty when unset). The two must compare
// equal or the first `pulumi up` after an import replaces every imported resource.
func TestProviderConfigDiffIgnoresUncheckedImportInputs(t *testing.T) {
	server := newUnconfiguredProviderServer(t)
	raw := property.NewMap(map[string]property.Value{
		"customerBaseUrl": property.New("http://127.0.0.1:18080"),
		"version":         property.New("0.1.0-alpha.0+dev"),
		"__internal": property.New(map[string]property.Value{
			"pluginDownloadURL": property.New("github://api.github.com/jflavan/pulumi-osano"),
		}),
	})
	checked, err := server.CheckConfig(p.CheckRequest{Urn: providerConfigURN(), Inputs: raw})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.DiffConfig(p.DiffRequest{Urn: providerConfigURN(), State: raw, Inputs: checked.Inputs})
	if err != nil {
		t.Fatal(err)
	}
	if resp.HasChanges {
		t.Fatalf("checked inputs must not diff against the unchecked inputs recorded by pulumi import: %#v",
			resp.DetailedDiff)
	}
}

func TestProviderConfigDiffReportsNoChangesForEqualConfig(t *testing.T) {
	server := newUnconfiguredProviderServer(t)
	config := property.NewMap(map[string]property.Value{
		"customerBaseUrl":       property.New("https://api.osano.com"),
		"osanoApiKey":           property.New("key").WithSecret(true),
		"requestTimeoutSeconds": property.New(30.0),
	})
	resp, err := server.DiffConfig(p.DiffRequest{Urn: providerConfigURN(), State: config, Inputs: config})
	if err != nil {
		t.Fatal(err)
	}
	if resp.HasChanges {
		t.Fatalf("identical provider config must not diff: %#v", resp.DetailedDiff)
	}
}
