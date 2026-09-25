//go:build e2e && pipeline

package pipeline

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jflavan/pulumi-osano/tests/e2e/internal/testenv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const (
	envRunPipelineE2E = "OSANO_RUN_PIPELINE_E2E"

	// Unusual prerelease versions keep these test builds from colliding with any real release.
	versionV1 = "0.0.0-e2e.1"
	versionV2 = "0.0.0-e2e.2"

	suiteTimeout = 15 * time.Minute
)

// TestPipeline runs real Pulumi engine operations (up, preview, refresh, destroy) on a Pulumi YAML
// program that uses the provider plugin built from this repository, against a mock Osano API.
func TestPipeline(t *testing.T) {
	testenv.RequireOptIn(t, envRunPipelineE2E, "1 to run the Pulumi engine pipeline suite against a local mock")

	ctx, cancel := context.WithTimeout(t.Context(), suiteTimeout)
	defer cancel()
	h := newHarness(ctx, t, versionV1, versionV2)

	t.Run("DefaultProviders", func(t *testing.T) { testDefaultProviderLifecycle(ctx, t, h) })
	t.Run("ExplicitProviderUpgrade", func(t *testing.T) { testExplicitProviderUpgrade(ctx, t, h) })
}

func scriptSrcFor(configID string) string {
	return fmt.Sprintf("https://cmp.osano.com/%s/%s/osano.js", mockCustomerID, configID)
}

func scriptTagFor(configID string) string {
	return `<script src="` + scriptSrcFor(configID) + `"></script>`
}

// requireScriptFirstInHead fails unless scriptTag is the first element inside <head>.
func requireScriptFirstInHead(t *testing.T, html, scriptTag string) {
	t.Helper()
	_, head, found := strings.Cut(html, "<head>")
	if !found {
		t.Errorf("indexHtml has no <head>:\n%s", html)
		return
	}
	if !strings.HasPrefix(strings.TrimSpace(head), scriptTag) {
		t.Errorf("indexHtml does not start <head> with the CMP script %s:\n%s", scriptTag, html)
	}
	if strings.Contains(scriptTag, "async") || strings.Contains(scriptTag, "defer") {
		t.Errorf("the CMP script tag must load synchronously, got %s", scriptTag)
	}
}

// requireNoChanges fails unless every resource step was "same" (after any refresh steps).
func (r *opResult) requireNoChanges(t *testing.T) bool {
	t.Helper()
	converged := true
	for _, step := range r.Steps {
		for _, op := range step.Ops {
			if op != apitype.OpSame && op != apitype.OpRefresh {
				converged = false
				t.Errorf("%s: expected no changes, but %s", r.Label, step)
			}
		}
	}
	if n := r.count(isWrite); n != 0 {
		converged = false
		t.Errorf("%s: expected no Osano writes, got %d:\n%s", r.Label, n, describeRequests(r.Requests))
	}
	return converged
}

func (r *opResult) requireCount(t *testing.T, what string, filter requestFilter, want int) {
	t.Helper()
	if got := r.count(filter); got != want {
		t.Errorf("%s: %s requests = %d, want %d\n%s", r.Label, what, got, want, describeRequests(r.Requests))
	}
}

// providerVersions maps each osano provider reference in the stack state to its version input.
func providerVersions(resources []apitype.ResourceV3) map[string]string {
	versions := map[string]string{}
	for i := range resources {
		res := &resources[i]
		if string(res.Type) != typeProvider {
			continue
		}
		version, _ := res.Inputs["version"].(string)
		versions[string(res.URN)+"::"+string(res.ID)] = version
	}
	return versions
}

// requireOsanoResourcesOnVersion fails unless every Osano resource in state uses a provider of version.
func requireOsanoResourcesOnVersion(t *testing.T, resources []apitype.ResourceV3, version string) {
	t.Helper()
	versions := providerVersions(resources)
	for i := range resources {
		res := &resources[i]
		if !strings.HasPrefix(string(res.Type), "osano:") {
			continue
		}
		if got, ok := versions[res.Provider]; !ok || got != version {
			t.Errorf("%s uses provider %s (version %q), want version %s; providers in state: %v",
				res.URN, res.Provider, got, version, versions)
		}
	}
}

// defaultProviderName is the engine's name for the default provider of a pinned version.
func defaultProviderName(version string) string {
	return "default_" + strings.NewReplacer(".", "_", "-", "_").Replace(version)
}

func paletteValue(configuration map[string]any, key string) any {
	palette, _ := configuration["palette"].(map[string]any)
	return palette[key]
}

func paletteColor(configuration map[string]any) any {
	return paletteValue(configuration, "buttonBackgroundColor")
}

// testDefaultProviderLifecycle walks one stack through its whole life; each step depends on the state
// the previous one left behind.
func testDefaultProviderLifecycle(ctx context.Context, t *testing.T, h *harness) {
	mock := newMockOsano(t)
	p := h.newProject(ctx, t, mock, "default-providers.Pulumi.yaml", versionV1)
	p.setConfig(ctx, t, map[string]configValue{
		"osano:customerBaseUrl": {Value: mock.URL()},
		"osano:osanoApiKey":     {Value: mockAPIKey, Secret: true},
		"changeToken":           {Value: "release-1"},
		"buttonColor":           {Value: "#000000"},
		"ruleTitle":             {Value: "Google Analytics"},
	})

	var configID string
	var ruleID int

	if !t.Run("a_UpCreatesAndPublishesOnce", func(t *testing.T) {
		res := p.up(ctx, t, "up #1 (create)")
		res.requireOps(t, typeConfig, apitype.OpCreate)
		res.requireOps(t, typeRule, apitype.OpCreate)
		res.requireOps(t, typePublication, apitype.OpCreate)
		res.requireUserAgent(t, versionV1)
		res.requireCount(t, "publish", isPublish, 1)
		res.requireCount(t, "config create", isConfigCreate, 1)
		res.requireCount(t, "rule create", isRuleCreate, 1)

		ids := mock.ConfigIDs()
		if len(ids) != 1 {
			t.Fatalf("expected the mock to hold one config, got %v", ids)
		}
		configID = ids[0]
		if got := res.output(t, "configId"); got != configID {
			t.Errorf("configId output = %q, want %q", got, configID)
		}
		wantTag := scriptTagFor(configID)
		scriptTag := res.output(t, "scriptTag")
		if scriptTag != wantTag {
			t.Errorf("scriptTag = %q, want %q", scriptTag, wantTag)
		}
		if got := res.output(t, "scriptSrc"); got != scriptSrcFor(configID) {
			t.Errorf("scriptSrc = %q, want %q", got, scriptSrcFor(configID))
		}
		if got := res.output(t, "customerId"); got != mockCustomerID {
			t.Errorf("customerId = %q, want %q", got, mockCustomerID)
		}
		if got := res.output(t, "lookupScriptTag"); got != scriptTag {
			t.Errorf("getCookieConsentConfig scriptTag = %q, want the publication's %q", got, scriptTag)
		}
		if got := res.output(t, "lookupPublishStatus"); got != statusPublished {
			t.Errorf("getCookieConsentConfig publishStatus = %q, want %q", got, statusPublished)
		}
		requireScriptFirstInHead(t, res.output(t, "indexHtml"), wantTag)
		// The program sees Osano's configuration with server defaults at the top level and inside palette.
		reported := res.outputMap(t, "lookupConfiguration")
		if reported["showWidget"] != true || paletteValue(reported, "buttonForegroundColor") != "#FFFFFF" ||
			paletteColor(reported) != "#000000" {
			t.Errorf("getCookieConsentConfig configuration should merge server defaults with the program's "+
				"values, got %v", reported)
		}

		config, _ := mock.Config(configID)
		if config.PublishStatus != statusPublished || config.PublishedRevision == 0 || config.LastPublished == 0 {
			t.Errorf("mock config after first publish: %+v", config)
		}
		if keep, _ := config.LastPublishBody["keepUnclassifiedTattles"].(bool); !keep {
			t.Errorf("publish body = %v, want keepUnclassifiedTattles=true", config.LastPublishBody)
		}
		rules := mock.Rules()
		if len(rules) != 1 || rules[0].ConfigID != configID {
			t.Fatalf("expected one rule on config %s, got %+v", configID, rules)
		}
		ruleID = rules[0].RuleID
	}) {
		t.FailNow()
	}

	t.Run("b_UnchangedUpIsANoOp", func(t *testing.T) {
		res := p.up(ctx, t, "up #2 (no changes)", "--expect-no-changes")
		res.requireAllSame(t)
		res.requireCount(t, "publish", isPublish, 0)
		res.requireCount(t, "write", isWrite, 0)
		res.requireUserAgent(t, versionV1)
	})

	t.Run("c_ConfigAndTokenChangeRepublishesOnce", func(t *testing.T) {
		before, _ := mock.Config(configID)
		publicationBefore := resourceOfType(t, p.resources(ctx, t), typePublication)
		p.setConfig(ctx, t, map[string]configValue{
			"buttonColor": {Value: "#0B5FFF"},
			"changeToken": {Value: "release-2"},
		})

		pre := p.preview(ctx, t, "preview #3 (config value + changeToken)")
		pre.requireOps(t, typeConfig, apitype.OpUpdate)
		pre.requireOps(t, typeRule, apitype.OpSame)
		pre.requireOps(t, typePublication, apitype.OpUpdate)
		pre.requireNoOsanoReplacement(t)
		pre.requireCount(t, "write (during preview)", isWrite, 0)

		res := p.up(ctx, t, "up #3 (config value + changeToken)")
		res.requireOps(t, typeConfig, apitype.OpUpdate)
		res.requireOps(t, typeRule, apitype.OpSame)
		res.requireOps(t, typePublication, apitype.OpUpdate)
		res.requireNoOsanoReplacement(t)
		res.requireCount(t, "publish", isPublish, 1)
		res.requireCount(t, "config PATCH", isConfigPatch, 1)
		res.requireCount(t, "config create", isConfigCreate, 0)
		res.requireCount(t, "rule create", isRuleCreate, 0)
		res.requireUserAgent(t, versionV1)

		after, _ := mock.Config(configID)
		if got := paletteColor(after.Configuration); got != "#0B5FFF" {
			t.Errorf("Osano palette.buttonBackgroundColor = %v, want #0B5FFF", got)
		}
		if after.PublishStatus != statusPublished || after.PublishedRevision <= before.PublishedRevision ||
			after.LastPublished <= before.LastPublished {
			t.Errorf("expected a completed republish: before %+v, after %+v", before, after)
		}
		publicationAfter := resourceOfType(t, p.resources(ctx, t), typePublication)
		if publicationAfter.URN != publicationBefore.URN || publicationAfter.ID != publicationBefore.ID {
			t.Errorf("publication identity changed: %s/%s -> %s/%s", publicationBefore.URN, publicationBefore.ID,
				publicationAfter.URN, publicationAfter.ID)
		}
		if got := res.output(t, "scriptTag"); got != scriptTagFor(configID) {
			t.Errorf("scriptTag changed to %q", got)
		}
		if got := res.output(t, "lookupPublishStatus"); got != statusPublished {
			t.Errorf("getCookieConsentConfig publishStatus = %q, want %q", got, statusPublished)
		}
	})

	t.Run("d_RuleOnlyChangeDoesNotPublish", func(t *testing.T) {
		p.setConfig(ctx, t, map[string]configValue{"ruleTitle": {Value: "Google Analytics 4"}})
		res := p.up(ctx, t, "up #4 (rule title only)")
		res.requireOps(t, typeConfig, apitype.OpSame)
		res.requireOps(t, typeRule, apitype.OpUpdate)
		res.requireOps(t, typePublication, apitype.OpSame)
		res.requireNoOsanoReplacement(t)
		res.requireCount(t, "rule PATCH", isRulePatch, 1)
		res.requireCount(t, "publish", isPublish, 0)
		res.requireCount(t, "config PATCH", isConfigPatch, 0)
		res.requireUserAgent(t, versionV1)

		rules := mock.Rules()
		if len(rules) != 1 || rules[0].RuleID != ruleID || rules[0].Title == nil ||
			*rules[0].Title != "Google Analytics 4" {
			t.Errorf("expected rule %d retitled in place, got %+v", ruleID, rules)
		}
		for _, request := range res.Requests {
			if isRulePatch(request) && request.Path != rulesPath+"/"+strconv.Itoa(ruleID) {
				t.Errorf("rule PATCH went to %s, want rule %d", request.Path, ruleID)
			}
		}
	})

	t.Run("e_RefreshConvergesDespiteServerDefaults", func(t *testing.T) {
		// Osano rolls out new defaults after the config exists: one top-level key and one key inside
		// the palette object the program declares partially.
		mock.AddServerDefaults(map[string]any{
			"crossDomain": false,
			"palette":     map[string]any{"linkColor": "#0645AD"},
		})
		writesBefore := countRequests(mock.RequestsSince(0), isWrite)

		ref := p.refresh(ctx, t, "refresh #5 (server defaults present)")
		ref.requireCount(t, "write (during refresh)", isWrite, 0)
		// Refresh keeps only the keys the program declares, at every level of the configuration object.
		refreshed := resourceOfType(t, p.resources(ctx, t), typeConfig)
		declared := map[string]any{
			"storagePolicyHref": "https://www.example.com/privacy",
			"palette":           map[string]any{"buttonBackgroundColor": "#0B5FFF"},
		}
		for _, side := range []struct {
			name  string
			value any
		}{{"inputs", refreshed.Inputs["configuration"]}, {"outputs", refreshed.Outputs["configuration"]}} {
			if !reflect.DeepEqual(side.value, declared) {
				t.Errorf("refreshed config %s.configuration = %v, want only the declared keys %v",
					side.name, side.value, declared)
			}
		}

		pre := p.preview(ctx, t, "preview #5 (after refresh)")
		if !pre.requireNoChanges(t) {
			t.Logf("preview output after refresh:\n%s", pre.StdOut)
		}

		upRefresh := p.up(ctx, t, "up --refresh #5", "--refresh")
		if !upRefresh.requireNoChanges(t) {
			t.Logf("up --refresh output:\n%s", upRefresh.StdOut)
			// Show whether the diff is perpetual: a second refresh against unchanged Osano state.
			again := p.preview(ctx, t, "preview --refresh #5b (perpetual diff check)", "--refresh")
			if again.requireNoChanges(t) {
				t.Logf("the second refresh converged; the diff was one-off")
			} else {
				t.Errorf("PERPETUAL DIFF: every `up --refresh` re-applies the configuration although nothing changed")
			}
		}
		if writes := countRequests(mock.RequestsSince(0), isWrite) - writesBefore; writes != 0 {
			t.Errorf("refresh scenario caused %d Osano writes", writes)
		}
		// The same run read Osano's new defaults, so convergence is not an artifact of stale reads.
		reported := upRefresh.outputMap(t, "lookupConfiguration")
		if reported["crossDomain"] != false || paletteValue(reported, "linkColor") != "#0645AD" {
			t.Errorf("Osano should report the new server defaults, got %v", reported)
		}
	})

	t.Run("e2_RefreshStillDetectsDriftInADeclaredNestedKey", func(t *testing.T) {
		// Someone changes a palette key the program declares in the Osano dashboard.
		mock.EditPalette(t, configID, "buttonBackgroundColor", "#FF0000")
		res := p.up(ctx, t, "up --refresh #5c (dashboard edited palette.buttonBackgroundColor)", "--refresh")
		if !res.hasOp(typeConfig, apitype.OpUpdate) {
			t.Errorf("%s: dashboard drift in a declared nested key must be corrected:\n%s", res.Label, res.describeSteps())
		}
		for _, typ := range []string{typeRule, typePublication} {
			if res.hasOp(typ, apitype.OpUpdate) {
				t.Errorf("%s: %s should not change:\n%s", res.Label, typ, res.describeSteps())
			}
		}
		res.requireNoOsanoReplacement(t)
		res.requireCount(t, "config PATCH", isConfigPatch, 1)
		res.requireCount(t, "publish", isPublish, 0)
		after, _ := mock.Config(configID)
		if got := paletteColor(after.Configuration); got != "#0B5FFF" {
			t.Errorf("Osano palette.buttonBackgroundColor = %v after the correcting update, want #0B5FFF", got)
		}
	})

	t.Run("f_ProviderVersionBumpUsesNewPlugin", func(t *testing.T) {
		p.writeProgram(t, versionV2)

		// Default providers are named after the version they pin, so the engine creates the new default
		// provider and deletes the old one; the Osano resources move to it without a diff or replacement.
		requireDefaultProviderSwap := func(r *opResult) {
			t.Helper()
			ops := map[string][]apitype.OpType{}
			for _, step := range r.stepsOfType(typeProvider) {
				ops[step.URN[strings.LastIndex(step.URN, "::")+2:]] = step.Ops
			}
			want := map[string][]apitype.OpType{
				defaultProviderName(versionV1): {apitype.OpDelete},
				defaultProviderName(versionV2): {apitype.OpCreate},
			}
			if !reflect.DeepEqual(ops, want) {
				t.Errorf("%s: default provider steps = %v, want %v", r.Label, ops, want)
			}
			r.requireNoOsanoReplacement(t)
			for _, typ := range []string{typeConfig, typeRule, typePublication} {
				r.requireOps(t, typ, apitype.OpSame)
			}
		}

		pre := p.preview(ctx, t, "preview #6 (provider "+versionV1+" -> "+versionV2+")")
		requireDefaultProviderSwap(pre)
		pre.requireCount(t, "write (during preview)", isWrite, 0)

		res := p.up(ctx, t, "up #6 (provider "+versionV1+" -> "+versionV2+")")
		requireDefaultProviderSwap(res)
		res.requireCount(t, "write", isWrite, 0)
		// The invoke runs on every up; after the bump it must be served by the new plugin.
		if len(res.Requests) == 0 {
			t.Errorf("%s: expected the getCookieConsentConfig invoke to call Osano", res.Label)
		}
		res.requireUserAgent(t, versionV2)
		requireOsanoResourcesOnVersion(t, p.resources(ctx, t), versionV2)

		ref := p.refresh(ctx, t, "refresh #6 (after provider bump)")
		if len(ref.Requests) == 0 {
			t.Errorf("%s: expected reads from Osano", ref.Label)
		}
		ref.requireUserAgent(t, versionV2)

		p.setConfig(ctx, t, map[string]configValue{"changeToken": {Value: "release-3"}})
		republish := p.up(ctx, t, "up #7 (changeToken on the new plugin)")
		republish.requireOps(t, typePublication, apitype.OpUpdate)
		republish.requireNoOsanoReplacement(t)
		republish.requireCount(t, "publish", isPublish, 1)
		republish.requireUserAgent(t, versionV2)
	})

	t.Run("g_DestroyDeletesRulesAndKeepsConfig", func(t *testing.T) {
		res := p.destroy(ctx, t, "destroy")
		res.requireOps(t, typeConfig, apitype.OpDelete)
		res.requireOps(t, typeRule, apitype.OpDelete)
		res.requireOps(t, typePublication, apitype.OpDelete)
		res.requireCount(t, "rule DELETE", isRuleDelete, 1)
		res.requireCount(t, "config DELETE", isConfigDelete, 0)
		res.requireCount(t, "publish", isPublish, 0)
		res.requireCount(t, "write", isWrite, 1)
		for _, request := range res.Requests {
			if isRuleDelete(request) && request.Path != rulesPath+"/"+strconv.Itoa(ruleID) {
				t.Errorf("rule DELETE went to %s, want rule %d", request.Path, ruleID)
			}
		}
		res.requireUserAgent(t, versionV2)

		if remaining := p.resources(ctx, t); len(remaining) != 0 {
			t.Errorf("stack still holds %d resources after destroy", len(remaining))
		}
		if rules := mock.Rules(); len(rules) != 0 {
			t.Errorf("rules left in Osano after destroy: %+v", rules)
		}
		if _, ok := mock.Config(configID); !ok {
			t.Errorf("config %s should be retained upstream after destroy", configID)
		}
	})
}

func testExplicitProviderUpgrade(ctx context.Context, t *testing.T, h *harness) {
	mock := newMockOsano(t)
	p := h.newProject(ctx, t, mock, "explicit-provider.Pulumi.yaml", versionV1)
	p.setConfig(ctx, t, map[string]configValue{
		"changeToken":  {Value: "explicit-1"},
		"osanoBaseUrl": {Value: mock.URL()},
		"osanoKey":     {Value: mockAPIKey, Secret: true},
	})

	create := p.up(ctx, t, "explicit up #1 (create on "+versionV1+")")
	create.requireOps(t, typeProvider, apitype.OpCreate)
	create.requireOps(t, typeConfig, apitype.OpCreate)
	create.requireOps(t, typePublication, apitype.OpCreate)
	create.requireCount(t, "publish", isPublish, 1)
	create.requireUserAgent(t, versionV1)
	ids := mock.ConfigIDs()
	if len(ids) != 1 {
		t.Fatalf("expected one config in the mock, got %v", ids)
	}
	if got := create.output(t, "scriptTag"); got != scriptTagFor(ids[0]) {
		t.Errorf("scriptTag = %q, want %q", got, scriptTagFor(ids[0]))
	}
	if got, want := create.output(t, "lookupScriptTag"), create.output(t, "scriptTag"); got != want {
		t.Errorf("getCookieConsentConfig scriptTag = %q, want %q", got, want)
	}
	providerBefore := resourceOfType(t, p.resources(ctx, t), typeProvider)

	p.writeProgram(t, versionV2)
	pre := p.preview(ctx, t, "explicit preview #2 (provider "+versionV1+" -> "+versionV2+")")
	pre.requireOps(t, typeProvider, apitype.OpUpdate)
	pre.requireOps(t, typeConfig, apitype.OpSame)
	pre.requireOps(t, typePublication, apitype.OpSame)
	pre.requireNoOsanoReplacement(t)
	for _, step := range pre.stepsOfType(typeProvider) {
		if _, ok := step.DetailedDiff["version"]; !ok && !slices.Contains(step.Diffs, "version") {
			t.Errorf("provider update should be attributed to its version input: %s", step)
		}
	}

	upgrade := p.up(ctx, t, "explicit up #2 (provider "+versionV1+" -> "+versionV2+")")
	upgrade.requireOps(t, typeProvider, apitype.OpUpdate)
	upgrade.requireOps(t, typeConfig, apitype.OpSame)
	upgrade.requireOps(t, typePublication, apitype.OpSame)
	upgrade.requireNoOsanoReplacement(t)
	upgrade.requireCount(t, "write", isWrite, 0)
	if len(upgrade.Requests) == 0 {
		t.Errorf("%s: expected the getCookieConsentConfig invoke to call Osano", upgrade.Label)
	}
	upgrade.requireUserAgent(t, versionV2)

	resources := p.resources(ctx, t)
	providerAfter := resourceOfType(t, resources, typeProvider)
	if providerAfter.URN != providerBefore.URN || providerAfter.ID != providerBefore.ID {
		t.Errorf("explicit provider was recreated: %s/%s -> %s/%s",
			providerBefore.URN, providerBefore.ID, providerAfter.URN, providerAfter.ID)
	}
	requireOsanoResourcesOnVersion(t, resources, versionV2)

	ref := p.refresh(ctx, t, "explicit refresh #3 (after provider bump)")
	if len(ref.Requests) == 0 {
		t.Errorf("%s: expected reads from Osano", ref.Label)
	}
	ref.requireUserAgent(t, versionV2)

	destroy := p.destroy(ctx, t, "explicit destroy")
	destroy.requireCount(t, "write", isWrite, 0)
	if remaining := p.resources(ctx, t); len(remaining) != 0 {
		t.Errorf("stack still holds %d resources after destroy", len(remaining))
	}
}
