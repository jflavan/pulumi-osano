package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func queryValue(t *testing.T, rawQuery, key string) string {
	t.Helper()
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	return values.Get(key)
}

// Rules added in 0.2.0 must not break a stack whose unchanged Consent Osano already accepted.
func TestConsentCheckAppliesValueRulesOnlyToChangedValues(t *testing.T) {
	server := newUCProviderServer(t, "https://uc.invalid")
	inputs := func(origin, action string) property.Map {
		return property.NewMap(map[string]property.Value{
			"subject": property.New(map[string]property.Value{"verifiedId": property.New("user-1")}),
			"origin":  property.New(origin),
			"actions": property.New([]property.Value{property.New(map[string]property.Value{
				"target": property.New("t"), "vendor": property.New("v"), "action": property.New(action),
			})}),
		})
	}
	legacy := inputs("API", "accept")

	unchanged, err := server.Check(p.CheckRequest{Urn: cmpURN("Consent", "c"), State: legacy, Inputs: legacy})
	if err != nil {
		t.Fatal(err)
	}
	if len(unchanged.Failures) != 0 {
		t.Fatalf("an unchanged consent must keep previewing, got %#v", unchanged.Failures)
	}

	created, err := server.Check(p.CheckRequest{Urn: cmpURN("Consent", "c"), Inputs: legacy})
	if err != nil {
		t.Fatal(err)
	}
	properties := map[string]bool{}
	for _, failure := range created.Failures {
		properties[failure.Property] = true
	}
	if !properties["origin"] || !properties["actions"] {
		t.Fatalf("a new consent must be checked, got %#v", created.Failures)
	}

	changed, err := server.Check(p.CheckRequest{
		Urn: cmpURN("Consent", "c"), State: legacy, Inputs: inputs("API", "reject"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changed.Failures) != 1 || changed.Failures[0].Property != "actions" {
		t.Fatalf("a changed action must be checked, got %#v", changed.Failures)
	}
}

func TestConsentCheckValidatesGeoOverrides(t *testing.T) {
	server := newUCProviderServer(t, "https://uc.invalid")
	base := property.NewMap(map[string]property.Value{
		"subject": property.New(map[string]property.Value{"verifiedId": property.New("user-1")}),
		"actions": property.New([]property.Value{property.New(map[string]property.Value{
			"target": property.New("t"), "vendor": property.New("v"), "action": property.New("ACCEPT"),
		})}),
	})
	for _, tc := range []struct {
		key, value, wantFailure string
	}{
		{"countryCodeOverride", "US", ""},
		{"countryCodeOverride", "USA", "countryCodeOverride"},
		{"regionCodeOverride", "US-CA", ""},
		{"regionCodeOverride", "CA", "regionCodeOverride"},
	} {
		resp, err := server.Check(p.CheckRequest{
			Urn: cmpURN("Consent", "c"), Inputs: base.Set(tc.key, property.New(tc.value)),
		})
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case tc.wantFailure == "" && len(resp.Failures) != 0:
			t.Fatalf("%s=%q: unexpected failures %#v", tc.key, tc.value, resp.Failures)
		case tc.wantFailure != "" && (len(resp.Failures) != 1 || resp.Failures[0].Property != tc.wantFailure):
			t.Fatalf("%s=%q: expected a %s failure, got %#v", tc.key, tc.value, tc.wantFailure, resp.Failures)
		}
	}

	// A malformed code would make Osano answer 400, which a lookup reads as "no consent".
	_, requests, err := invokeUC(t, "getUnifiedConsent", map[string]property.Value{
		"subjectRef": property.New("s-1"), "countryCodeOverride": property.New("Germany"),
	}, http.StatusOK, nil)
	if err == nil || !strings.Contains(err.Error(), "countryCodeOverride") {
		t.Fatalf("expected a countryCodeOverride error, got %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("expected no request, got %#v", requests)
	}
}

func TestGPCConsentReportsIgnoredInputs(t *testing.T) {
	t.Parallel()
	token := "session-1"
	args := ConsentArgs{
		Origin:       consentOriginGPC,
		Tags:         []string{"ccpa"},
		SessionToken: &token,
		Compliance:   &ConsentCompliance{PrivacyPolicy: &ConsentPrivacyPolicy{URL: "https://example.com"}},
	}
	want := []string{"tags", "sessionToken", "compliance.privacyPolicy"}
	if got := args.gpcIgnoredInputs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if got := (ConsentArgs{Origin: consentOriginGPC}).gpcIgnoredInputs(); len(got) != 0 {
		t.Fatalf("expected nothing ignored, got %v", got)
	}
}

func TestGetUnifiedConsentKeepsEmptyLists(t *testing.T) {
	resp, _, err := invokeUC(t, "getUnifiedConsent", map[string]property.Value{"subjectRef": property.New("s-1")},
		http.StatusOK, map[string]any{"unifiedConsent": map[string]any{"subjectId": "s-1", "tags": []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	tags := resp.Return.Get("unifiedConsent").AsMap().Get("tags")
	if !tags.IsArray() || tags.AsArray().Len() != 0 {
		t.Fatalf("expected an empty tags list, got %#v", tags)
	}
}

func TestEmailVerificationDoesNotSendTheSession(t *testing.T) {
	_, requests, err := invokeUC(t, "verifySubjectCode", map[string]property.Value{
		"email": property.New("person@example.com"), "code": property.New("123456"), "session": property.New("s"),
	}, http.StatusOK, map[string]any{"verifiedId": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if _, sent := requests[0].Body["session"]; sent {
		t.Fatalf("session must be sent only with SMS verification, got %#v", requests[0].Body)
	}
}

// A request that gets no response must not print its URL, whose path can hold a secret session ID.
func TestTransportErrorsDoNotRevealTheRequestPath(t *testing.T) {
	api := httptest.NewServer(http.NotFoundHandler())
	baseURL := api.URL
	api.Close()

	server := newUCProviderServer(t, baseURL)
	_, err := server.Invoke(p.InvokeRequest{
		Token: tokens.Type("osano:index:" + "getSession"),
		Args:  property.NewMap(map[string]property.Value{"sessionId": property.New("SUPER-SECRET-SESSION")}),
	})
	if err == nil {
		t.Fatal("expected the request to fail")
	}
	if strings.Contains(err.Error(), "SUPER-SECRET-SESSION") || strings.Contains(err.Error(), "/v2/sessions") {
		t.Fatalf("error reveals the request path: %v", err)
	}
	if !strings.Contains(err.Error(), "Osano API request failed: GET") {
		t.Fatalf("expected a transport error, got %v", err)
	}
}

func TestCookieConsentConfigCheckDoesNotBlockUnchangedConfigurations(t *testing.T) {
	t.Parallel()
	resource := &CookieConsentConfig{}
	inputs := func(sampling float64) property.Map {
		return property.NewMap(map[string]property.Value{
			"name":    property.New("cookie-consent"),
			"domains": property.New([]property.Value{property.New("example.com")}),
			"mode":    property.New("production"),
			"configuration": property.New(map[string]property.Value{
				"storagePolicyHref": property.New("https://example.com/privacy"),
				"tattleSampling":    property.New(sampling),
			}),
		})
	}

	unchanged, err := resource.Check(context.Background(), infer.CheckRequest{
		OldInputs: inputs(2), NewInputs: inputs(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unchanged.Failures) != 0 {
		t.Fatalf("an unchanged configuration must keep previewing, got %#v", unchanged.Failures)
	}

	changed, err := resource.Check(context.Background(), infer.CheckRequest{
		OldInputs: inputs(0.5), NewInputs: inputs(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertFailureProperty(t, changed.Failures, "configuration.tattleSampling")
}

func TestValidateCookieConsentConfigurationEdgeCases(t *testing.T) {
	t.Parallel()
	with := func(palette map[string]any) map[string]any {
		return map[string]any{"storagePolicyHref": "/privacy", "palette": palette}
	}

	for name, palette := range map[string]map[string]any{
		"null enums":                            {"dialogType": nil, "theme": nil, "widgetPosition": nil},
		"box position without dialogType":       {"displayPosition": "center"},
		"bar position without dialogType":       {"displayPosition": "top"},
		"box position with a null dialogType":   {"dialogType": nil, "displayPosition": "bottom-right"},
		"box position with dialogType box":      {"dialogType": "box", "displayPosition": "top-left"},
		"null display position with dialogType": {"dialogType": "bar", "displayPosition": nil},
	} {
		if failures, _ := validateCookieConsentConfiguration(with(palette), "production", true); len(failures) != 0 {
			t.Errorf("%s: unexpected failures %#v", name, failures)
		}
	}
	for name, palette := range map[string]map[string]any{
		"unknown position without dialogType": {"displayPosition": "middle"},
		"box position with dialogType bar":    {"dialogType": "bar", "displayPosition": "center"},
	} {
		failures, _ := validateCookieConsentConfiguration(with(palette), "production", true)
		if len(failures) != 1 || failures[0].Property != "configuration.palette" {
			t.Errorf("%s: expected one palette failure, got %#v", name, failures)
		}
	}

	base := map[string]any{"storagePolicyHref": "/privacy"}
	warns := func(configuration map[string]any, creating bool) bool {
		_, warnings := validateCookieConsentConfiguration(configuration, "debug", creating)
		for _, warning := range warnings {
			if strings.Contains(warning, "googleConsent") {
				return true
			}
		}
		return false
	}
	if !warns(base, true) {
		t.Error("expected a Google Consent Mode warning when creating a debug configuration")
	}
	if warns(base, false) {
		t.Error("an existing debug configuration that does not declare googleConsent must not warn")
	}
	if !warns(map[string]any{"storagePolicyHref": "/privacy", "googleConsent": true}, false) {
		t.Error("expected a warning when googleConsent is declared true in debug mode")
	}
}

func TestProjectConfigurationKeepsClearedValues(t *testing.T) {
	t.Parallel()
	server := map[string]any{
		"variantMapping": nil,
		"palette":        map[string]any{"linkColor": "#37CD8F"},
	}
	declared := map[string]any{
		"variantMapping": map[string]any{},
		"palette":        map[string]any{"linkColor": nil},
	}
	want := map[string]any{
		"variantMapping": map[string]any{},
		"palette":        map[string]any{"linkColor": nil},
	}
	if got := projectConfiguration(server, declared); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleared values must not drift:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestProviderVersionDiffKinds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		previous, next property.Value
		wantKind       p.DiffKind
		wantChanged    bool
	}{
		{"unchanged", property.New("0.2.0"), property.New("0.2.0"), p.Update, false},
		{"both unset", property.Value{}, property.Value{}, p.Update, false},
		{"upgraded", property.New("0.1.0"), property.New("0.2.0"), p.Update, true},
		{"added", property.Value{}, property.New("0.2.0"), p.Add, true},
		{"removed", property.New("0.1.0"), property.Value{}, p.Delete, true},
	} {
		kind, changed := providerVersionDiff(tc.previous, tc.next)
		if changed != tc.wantChanged || (changed && kind != tc.wantKind) {
			t.Errorf("%s: got %v/%v, want %v/%v", tc.name, kind, changed, tc.wantKind, tc.wantChanged)
		}
	}
}

func TestCookieConsentListPageSizes(t *testing.T) {
	t.Run("rules ask for the documented maximum page", func(t *testing.T) {
		_, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs/config-123/rules": respondJSON(http.StatusOK, map[string]any{"items": []any{}}),
		}, "getCookieConsentRules", map[string]property.Value{"configId": property.New("config-123")})
		if err != nil {
			t.Fatal(err)
		}
		if got := queryValue(t, requests[0].RawQuery, "limit"); got != "500" {
			t.Fatalf("expected limit=500, got %q", got)
		}
	})

	t.Run("configs shrink the page to maxResults", func(t *testing.T) {
		_, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs": respondJSON(http.StatusOK, map[string]any{"items": []any{}}),
		}, "getCookieConsentConfigs", map[string]property.Value{"maxResults": property.New(3.0)})
		if err != nil {
			t.Fatal(err)
		}
		if got := queryValue(t, requests[0].RawQuery, "limit"); got != "3" {
			t.Fatalf("expected limit=3, got %q", got)
		}
	})

	t.Run("the audit log defaults to 200 events", func(t *testing.T) {
		_, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/audit-log": respondJSON(http.StatusOK, map[string]any{"items": []any{}}),
		}, "getCookieConsentAuditLog", map[string]property.Value{})
		if err != nil {
			t.Fatal(err)
		}
		if got := queryValue(t, requests[0].RawQuery, "limit"); got != "200" {
			t.Fatalf("expected limit=200, got %q", got)
		}
	})
}
