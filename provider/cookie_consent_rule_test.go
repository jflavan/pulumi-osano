package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestCookieConsentRuleCheck(t *testing.T) {
	r := &CookieConsentRule{}
	ctx := context.Background()

	t.Run("valid inputs", func(t *testing.T) {
		inputs := property.NewMap(validRuleCheckInputValues())
		resp, err := r.Check(ctx, infer.CheckRequest{NewInputs: inputs})
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
			name:       "missing configId",
			inputs:     property.NewMap(deleteKey(validRuleCheckInputValues(), "configId")),
			failureKey: "configId",
		},
		{
			name:       "missing storeType",
			inputs:     property.NewMap(deleteKey(validRuleCheckInputValues(), "storeType")),
			failureKey: "storeType",
		},
		{
			name:       "missing classification",
			inputs:     property.NewMap(deleteKey(validRuleCheckInputValues(), "classification")),
			failureKey: "classification",
		},
		{
			name:       "missing rule",
			inputs:     property.NewMap(deleteKey(validRuleCheckInputValues(), "rule")),
			failureKey: "rule",
		},
		{
			name: "invalid classification",
			inputs: property.NewMap(func() map[string]property.Value {
				v := validRuleCheckInputValues()
				v["classification"] = property.New("INVALID")
				return v
			}()),
			failureKey: "classification",
		},
		{
			name: "invalid storeType",
			inputs: property.NewMap(func() map[string]property.Value {
				v := validRuleCheckInputValues()
				v["storeType"] = property.New("invalid")
				return v
			}()),
			failureKey: "storeType",
		},
		{
			name: "rule too short",
			inputs: property.NewMap(func() map[string]property.Value {
				v := validRuleCheckInputValues()
				v["rule"] = property.New("ab")
				return v
			}()),
			failureKey: "rule",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := r.Check(ctx, infer.CheckRequest{NewInputs: tc.inputs})
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

func TestCookieConsentRuleDiff(t *testing.T) {
	r := &CookieConsentRule{}
	ctx := context.Background()

	t.Run("no changes", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
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

	t.Run("changed classification", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		inputs.Classification = "MARKETING"
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffUpdate(t, resp, "classification")
	})

	t.Run("changed rule", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		inputs.Rule = "new_pattern_*"
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffUpdate(t, resp, "rule")
	})

	t.Run("changed disclosure", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		inputs.Disclosure = true
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDiffUpdate(t, resp, "disclosure")
	})

	t.Run("configId change requires replace", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		inputs.ConfigId = "new-config-id"
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.HasChanges {
			t.Fatal("expected changes")
		}
		d, ok := resp.DetailedDiff["configId"]
		if !ok {
			t.Fatalf("expected diff for configId, got: %#v", resp.DetailedDiff)
		}
		if d.Kind != p.UpdateReplace {
			t.Fatalf("expected UpdateReplace for configId, got %v", d.Kind)
		}
	})

	t.Run("storeType change requires replace", func(t *testing.T) {
		state := baseRuleState()
		inputs := baseRuleArgs()
		inputs.StoreType = "iframes"
		resp, err := r.Diff(ctx, infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]{
			State:  state,
			Inputs: inputs,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.HasChanges {
			t.Fatal("expected changes")
		}
		d, ok := resp.DetailedDiff["storeType"]
		if !ok {
			t.Fatalf("expected diff for storeType, got: %#v", resp.DetailedDiff)
		}
		if d.Kind != p.UpdateReplace {
			t.Fatalf("expected UpdateReplace for storeType, got %v", d.Kind)
		}
	})
}

func TestPtrStringEqual(t *testing.T) {
	a := "hello"
	b := "hello"
	c := "world"

	cases := []struct {
		name string
		a    *string
		b    *string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, &a, false},
		{"b nil", &a, nil, false},
		{"equal", &a, &b, true},
		{"not equal", &a, &c, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ptrStringEqual(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("ptrStringEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func validRuleCheckInputValues() map[string]property.Value {
	return map[string]property.Value{
		"configId":       property.New("config-abc-123"),
		"storeType":      property.New("scripts"),
		"classification": property.New("ANALYTICS"),
		"rule":           property.New("google-analytics*"),
		"disclosure":     property.New(false),
	}
}

func baseRuleArgs() CookieConsentRuleArgs {
	return CookieConsentRuleArgs{
		ConfigId:       "config-abc-123",
		StoreType:      "scripts",
		Classification: "ANALYTICS",
		Rule:           "google-analytics*",
		Disclosure:     false,
	}
}

func baseRuleState() CookieConsentRuleState {
	return CookieConsentRuleState{
		CookieConsentRuleArgs: baseRuleArgs(),
		RuleId:                42,
	}
}
