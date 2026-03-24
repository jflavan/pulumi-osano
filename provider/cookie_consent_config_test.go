package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestCookieConsentConfigCheck(t *testing.T) {
	t.Parallel()

	resource := &CookieConsentConfig{}
	ctx := context.Background()

	t.Run("valid inputs", func(t *testing.T) {
		inputs := property.NewMap(validConfigCheckInputValues())
		resp, err := resource.Check(ctx, infer.CheckRequest{NewInputs: inputs})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Failures) != 0 {
			t.Fatalf("expected no failures, got: %#v", resp.Failures)
		}
	})

	cases := []struct {
		name       string
		inputs     property.Map
		failureKey string
	}{
		{
			name:       "missing name",
			inputs:     property.NewMap(deleteKey(validConfigCheckInputValues(), "name")),
			failureKey: "name",
		},
		{
			name:       "missing domains",
			inputs:     property.NewMap(deleteKey(validConfigCheckInputValues(), "domains")),
			failureKey: "domains",
		},
		{
			name:       "missing mode",
			inputs:     property.NewMap(deleteKey(validConfigCheckInputValues(), "mode")),
			failureKey: "mode",
		},
		{
			name: "missing storagePolicyHref",
			inputs: property.NewMap(func() map[string]property.Value {
				values := validConfigCheckInputValues()
				values["configuration"] = property.New(map[string]property.Value{
					"foo": property.New("bar"),
				})
				return values
			}()),
			failureKey: "configuration.storagePolicyHref",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := resource.Check(ctx, infer.CheckRequest{NewInputs: tc.inputs})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Failures) == 0 {
				t.Fatalf("expected failures for %s, got none", tc.failureKey)
			}
			assertFailureProperty(t, resp.Failures, tc.failureKey)
		})
	}
}

func TestCookieConsentConfigDiff(t *testing.T) {
	t.Parallel()

	resource := &CookieConsentConfig{}
	ctx := context.Background()

	t.Run("same inputs", func(t *testing.T) {
		state := baseConfigState()
		inputs := baseConfigArgs()
		resp, err := resource.Diff(ctx, infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.HasChanges {
			t.Fatalf("expected no changes, got: %#v", resp.DetailedDiff)
		}
	})

	t.Run("changed name", func(t *testing.T) {
		state := baseConfigState()
		inputs := baseConfigArgs()
		inputs.Name = "updated"
		resp, err := resource.Diff(ctx, infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffKind(t, resp, "name", p.Update)
	})

	t.Run("changed mode", func(t *testing.T) {
		state := baseConfigState()
		inputs := baseConfigArgs()
		inputs.Mode = "production"
		resp, err := resource.Diff(ctx, infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffKind(t, resp, "mode", p.Update)
	})

	t.Run("changed configuration key", func(t *testing.T) {
		state := baseConfigState()
		inputs := baseConfigArgs()
		inputs.Configuration = map[string]any{
			"storagePolicyHref": "https://example.com/policy",
			"flag":              false,
		}
		resp, err := resource.Diff(ctx, infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffKind(t, resp, "configuration", p.Update)
	})
}

func validConfigCheckInputValues() map[string]property.Value {
	return map[string]property.Value{
		"name": property.New("cookie-consent"),
		"domains": property.New([]property.Value{
			property.New("example.com"),
		}),
		"mode": property.New("debug"),
		"configuration": property.New(map[string]property.Value{
			"storagePolicyHref": property.New("https://example.com/storage-policy"),
		}),
	}
}

func deleteKey(values map[string]property.Value, key string) map[string]property.Value {
	delete(values, key)
	return values
}

func assertFailureProperty(t *testing.T, failures []p.CheckFailure, propertyKey string) {
	t.Helper()
	for _, failure := range failures {
		if failure.Property == propertyKey {
			return
		}
	}
	t.Fatalf("expected failure for %q, got %#v", propertyKey, failures)
}

func assertDiffKind(t *testing.T, resp p.DiffResponse, key string, want p.DiffKind) {
	t.Helper()
	if !resp.HasChanges {
		t.Fatalf("expected changes, got none")
	}
	diff, ok := resp.DetailedDiff[key]
	if !ok {
		t.Fatalf("expected diff for %q, got %#v", key, resp.DetailedDiff)
	}
	if diff.Kind != want {
		t.Fatalf("expected %q diff kind %v, got %v", key, want, diff.Kind)
	}
}

func baseConfigArgs() CookieConsentConfigArgs {
	return CookieConsentConfigArgs{
		Name:    "cookie-consent",
		Domains: []string{"example.com"},
		Mode:    "debug",
		OrgIDs:  []string{"org-123"},
		Configuration: map[string]any{
			"storagePolicyHref": "https://example.com/storage-policy",
			"flag":              true,
		},
	}
}

func baseConfigState() CookieConsentConfigState {
	return CookieConsentConfigState{CookieConsentConfigArgs: baseConfigArgs(), ConfigID: "config-123"}
}
