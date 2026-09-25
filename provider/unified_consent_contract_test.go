package provider

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// These tests pin the provider to Osano's Unified Consent Core API spec
// (https://developers.osano.com/uc/core-api/openapi).

func invokeUC(
	t *testing.T, token string, args map[string]property.Value, status int, response any,
) (p.InvokeResponse, []recordedUCRequest, error) {
	t.Helper()
	mock := &ucMockAPI{t: t, status: status, body: response}
	api := httptest.NewServer(mock)
	defer api.Close()

	server := newUCProviderServer(t, api.URL)
	resp, err := server.Invoke(p.InvokeRequest{
		Token: tokens.Type("osano:index:" + token),
		Args:  property.NewMap(args),
	})
	return resp, mock.recorded(), err
}

func TestUnifiedConsentReferenceTypes(t *testing.T) {
	cases := []struct {
		referenceType string
		wantRef       string
	}{
		{"", "subject"},
		{"subject", "subject"},
		{"session", "session"},
		{"anonymous", "subject"},
	}
	for _, tc := range cases {
		t.Run("getSubject "+tc.referenceType, func(t *testing.T) {
			args := map[string]property.Value{"subjectRef": property.New("ref-1")}
			if tc.referenceType != "" {
				args["referenceType"] = property.New(tc.referenceType)
			}
			_, requests, err := invokeUC(t, "getSubject", args, http.StatusOK, map[string]any{"id": "s"})
			if err != nil {
				t.Fatal(err)
			}
			if len(requests) != 1 || requests[0].Query["ref"] != tc.wantRef {
				t.Fatalf("expected ref=%s, got %#v", tc.wantRef, requests)
			}
		})
	}

	t.Run("an unknown reference type fails without a request", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "getUnifiedConsent", map[string]property.Value{
			"subjectRef": property.New("ref-1"), "referenceType": property.New("verified"),
		}, http.StatusOK, nil)
		if err == nil && len(resp.Failures) == 0 {
			t.Fatal("expected an invalid referenceType to fail")
		}
		if err != nil && !strings.Contains(err.Error(), "referenceType must be subject or session") {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(requests) != 0 {
			t.Fatalf("expected no request, got %#v", requests)
		}
	})
}

func TestUnifiedConsentGeoOverrides(t *testing.T) {
	geo := map[string]property.Value{
		"countryCodeOverride": property.New("DE"),
		"regionCodeOverride":  property.New("DE-BY"),
	}
	withGeo := func(args map[string]property.Value) map[string]property.Value {
		for k, v := range geo {
			args[k] = v
		}
		return args
	}
	cases := []struct {
		token    string
		args     map[string]property.Value
		response any
	}{
		{
			token:    "getUnifiedConsent",
			args:     withGeo(map[string]property.Value{"subjectRef": property.New("s-1")}),
			response: map[string]any{"unifiedConsent": map[string]any{"subjectId": "s-1"}},
		},
		{
			token:    "checkConsent",
			args:     withGeo(map[string]property.Value{"subjectId": property.New("s-1")}),
			response: map[string]any{"exists": true},
		},
		{
			token: "getConsentProfile",
			args: withGeo(map[string]property.Value{
				"hashedSubjectId": property.New("h-1"), "configId": property.New("c-1"),
			}),
			response: map[string]any{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			_, requests, err := invokeUC(t, tc.token, tc.args, http.StatusOK, tc.response)
			if err != nil {
				t.Fatal(err)
			}
			if len(requests) != 1 || requests[0].Country != "DE" || requests[0].Region != "DE-BY" {
				t.Fatalf("expected geolocation override headers, got %#v", requests)
			}
		})
	}
}

func TestGetUnifiedConsentReturnsSpecFields(t *testing.T) {
	resp, _, err := invokeUC(t, "getUnifiedConsent", map[string]property.Value{"subjectRef": property.New("s-1")},
		http.StatusOK, map[string]any{
			"unifiedConsent": map[string]any{
				"subjectId":        "s-1",
				"brandId":          "brand-1",
				"channelIds":       []any{"web", "app"},
				"lastUpdateDate":   "2026-09-01T00:00:00Z",
				"lastConflictDate": "2026-08-01T00:00:00Z",
				"actions":          []any{},
				"attributes":       map[string]any{},
			},
			"conflicts": []any{map[string]any{"type": "vendor", "resolution": "latest"}},
		})
	if err != nil {
		t.Fatal(err)
	}
	uc := resp.Return.Get("unifiedConsent").AsMap()
	assertString(t, uc, "brandId", "brand-1")
	assertString(t, uc, "lastConflictDate", "2026-08-01T00:00:00Z")
	if channels := uc.Get("channelIds").AsArray(); channels.Len() != 2 || channels.Get(1).AsString() != "app" {
		t.Fatalf("unexpected channelIds: %#v", uc.Get("channelIds"))
	}
	if resp.Return.Get("conflicts").AsArray().Len() != 1 {
		t.Fatalf("expected one conflict, got %#v", resp.Return.Get("conflicts"))
	}
}

func TestGetCollectionsRejectsUnknownType(t *testing.T) {
	resp, requests, err := invokeUC(t, "getCollections", map[string]property.Value{
		"type": property.New("archived"),
	}, http.StatusOK, nil)
	if err == nil && len(resp.Failures) == 0 {
		t.Fatal("expected an unknown collection type to fail")
	}
	if len(requests) != 0 {
		t.Fatalf("expected no request, got %#v", requests)
	}
}

func TestSubjectVerificationFollowsSpec(t *testing.T) {
	t.Run("email send-code omits hashedSubjectId when unset", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "sendSubjectCode", map[string]property.Value{
			"email": property.New("person@example.com"),
		}, http.StatusOK, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := recordedUCRequest{
			Method: http.MethodPost, Path: "/v2/subjects/send-code", Query: map[string]string{},
			Key: "test-osano-key", Body: map[string]any{"email": "person@example.com"},
		}
		if len(requests) != 1 || !reflect.DeepEqual(requests[0], want) {
			t.Fatalf("unexpected request: %#v", requests)
		}
		assertString(t, resp.Return, "session", "")
	})

	t.Run("SMS send-code returns the challenge session", func(t *testing.T) {
		resp, _, err := invokeUC(t, "sendSubjectCode", map[string]property.Value{
			"phone": property.New("+15555550100"),
		}, http.StatusOK, map[string]any{"session": "sms-session-1"})
		if err != nil {
			t.Fatal(err)
		}
		assertString(t, resp.Return, "session", "sms-session-1")
	})

	t.Run("SMS verify requires the session", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "verifySubjectCode", map[string]property.Value{
			"phone": property.New("+15555550100"), "code": property.New("12345678"),
		}, http.StatusOK, nil)
		if err == nil && len(resp.Failures) == 0 {
			t.Fatal("expected SMS verification without a session to fail")
		}
		if err != nil && !strings.Contains(err.Error(), "session is required") {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(requests) != 0 {
			t.Fatalf("expected no request, got %#v", requests)
		}
	})

	t.Run("SMS verify sends the session and returns the verified ID", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "verifySubjectCode", map[string]property.Value{
			"phone":   property.New("+15555550100"),
			"code":    property.New("12345678"),
			"session": property.New("sms-session-1"),
		}, http.StatusOK, map[string]any{"verifiedId": "verified-9"})
		if err != nil {
			t.Fatal(err)
		}
		want := recordedUCRequest{
			Method: http.MethodPost, Path: "/v2/subjects/profile/verify/sms", Query: map[string]string{},
			Key: "test-osano-key",
			Body: map[string]any{
				"phone": "+15555550100", "code": "12345678", "session": "sms-session-1",
			},
		}
		if len(requests) != 1 || !reflect.DeepEqual(requests[0], want) {
			t.Fatalf("unexpected request:\n got: %#v\nwant: %#v", requests, want)
		}
		assertString(t, resp.Return, "verifiedId", "verified-9")
		assertBool(t, resp.Return, "verified", true)
	})

	t.Run("subject routes fall back to the Unified Consent key without an Osano key", func(t *testing.T) {
		mock := &ucMockAPI{t: t}
		api := httptest.NewServer(mock)
		defer api.Close()

		for _, env := range []string{envOsanoAPIKey, envUnifiedConsent, envAPIBaseURL, envRequestTimeout} {
			t.Setenv(env, "")
		}
		server, err := integration.NewServer(
			t.Context(), Name, semver.MustParse("1.0.0"), integration.WithProvider(Provider()),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
			"unifiedConsentApiKey":  property.New("test-uc-key"),
			"apiBaseUrl":            property.New(api.URL),
			"requestTimeoutSeconds": property.New(2.0),
		})}); err != nil {
			t.Fatal(err)
		}
		if _, err := server.Invoke(p.InvokeRequest{
			Token: tokens.Type("osano:index:" + "sendSubjectCode"),
			Args:  property.NewMap(map[string]property.Value{"email": property.New("person@example.com")}),
		}); err != nil {
			t.Fatal(err)
		}
		requests := mock.recorded()
		if len(requests) != 1 || requests[0].Key != "" || requests[0].UCKey != "test-uc-key" {
			t.Fatalf("expected only the Unified Consent key, got %#v", requests)
		}
	})
}

func TestSubjectProfileAndSessionFunctions(t *testing.T) {
	t.Run("getSubjectProfile returns the email as a secret", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "getSubjectProfile", map[string]property.Value{
			"subjectId": property.New("subject/1"),
		}, http.StatusOK, map[string]any{"email": "person@example.com", "subjectId": "subject/1"})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || requests[0].Path != "/v2/subjects/subject%2F1/profile" ||
			requests[0].UCKey != "test-uc-key" {
			t.Fatalf("unexpected request: %#v", requests)
		}
		assertBool(t, resp.Return, "exists", true)
		email := resp.Return.Get("email")
		if !email.Secret() || email.AsString() != "person@example.com" {
			t.Fatalf("expected a secret email, got %#v", email)
		}
	})

	t.Run("getSubjectProfile maps 404 to a missing profile", func(t *testing.T) {
		resp, _, err := invokeUC(t, "getSubjectProfile", map[string]property.Value{
			"subjectId": property.New("missing"),
		}, http.StatusNotFound, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertBool(t, resp.Return, "exists", false)
	})

	t.Run("getSession resolves the verified ID", func(t *testing.T) {
		resp, requests, err := invokeUC(t, "getSession", map[string]property.Value{
			"sessionId": property.New("session-1"),
		}, http.StatusOK, map[string]any{
			"subject": map[string]any{"verifiedId": "verified-1"},
			"profile": map[string]any{"email": "person@example.com", "firstName": "Pat"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || requests[0].Path != "/v2/sessions/session-1" {
			t.Fatalf("unexpected request: %#v", requests)
		}
		assertBool(t, resp.Return, "exists", true)
		assertString(t, resp.Return, "verifiedId", "verified-1")
		if profile := resp.Return.Get("profile"); !profile.Secret() {
			t.Fatalf("expected a secret profile, got %#v", profile)
		}
	})
}

func TestConsentResourceFollowsSpec(t *testing.T) {
	baseInputs := func() property.Map {
		return property.NewMap(map[string]property.Value{
			"subject": property.New(map[string]property.Value{"anonymousId": property.New("anon-1")}),
			"actions": property.New([]property.Value{property.New(map[string]property.Value{
				"target": property.New("protocol-1"),
				"vendor": property.New("config-1"),
				"action": property.New("ACCEPT"),
			})}),
		})
	}
	create := func(t *testing.T, inputs property.Map, status int, response any) (
		p.CreateResponse, []recordedUCRequest, error,
	) {
		t.Helper()
		mock := &ucMockAPI{t: t, status: status, body: response}
		api := httptest.NewServer(mock)
		defer api.Close()
		server := newUCProviderServer(t, api.URL)
		resp, err := server.Create(p.CreateRequest{Urn: cmpURN("Consent", "c"), Properties: inputs})
		return resp, mock.recorded(), err
	}

	t.Run("attributes are always sent because Osano requires them", func(t *testing.T) {
		_, requests, err := create(t, baseInputs(), http.StatusCreated, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 {
			t.Fatalf("expected one request, got %#v", requests)
		}
		attributes, present := requests[0].Body["attributes"]
		if !present || !reflect.DeepEqual(attributes, map[string]any{}) {
			t.Fatalf("expected an empty attributes object, got %#v (present=%v)", attributes, present)
		}
	})

	t.Run("session token and geolocation overrides are sent", func(t *testing.T) {
		inputs := baseInputs().
			Set("sessionToken", property.New("token-1")).
			Set("countryCodeOverride", property.New("FR")).
			Set("regionCodeOverride", property.New("FR-IDF"))
		_, requests, err := create(t, inputs, http.StatusCreated, nil)
		if err != nil {
			t.Fatal(err)
		}
		if requests[0].Body["sessionToken"] != "token-1" || requests[0].Country != "FR" || requests[0].Region != "FR-IDF" {
			t.Fatalf("unexpected request: %#v", requests[0])
		}
	})

	t.Run("a GPC consent without actions uses the GPC endpoint and records the derived actions", func(t *testing.T) {
		inputs := property.NewMap(map[string]property.Value{
			"subject":    property.New(map[string]property.Value{"anonymousId": property.New("anon-1")}),
			"origin":     property.New("gpc"),
			"compliance": property.New(map[string]property.Value{"gpc": property.New(1.0)}),
		})
		resp, requests, err := create(t, inputs, http.StatusCreated, map[string]any{
			"gpcActions": []any{map[string]any{"target": "sale", "vendor": "config-1", "action": "REJECT"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || requests[0].Path != "/v2/consents/gpc" {
			t.Fatalf("expected the GPC endpoint, got %#v", requests)
		}
		wantBody := map[string]any{
			"subject":    map[string]any{"anonymousId": "anon-1"},
			"compliance": map[string]any{"gpc": float64(1)},
			"attributes": map[string]any{},
		}
		if !reflect.DeepEqual(requests[0].Body, wantBody) {
			t.Fatalf("unexpected GPC body:\n got: %#v\nwant: %#v", requests[0].Body, wantBody)
		}
		actions := resp.Properties.Get("gpcActions").AsArray()
		if actions.Len() != 1 || actions.Get(0).AsMap().Get("action").AsString() != "REJECT" {
			t.Fatalf("unexpected gpcActions: %#v", resp.Properties.Get("gpcActions"))
		}
	})

	t.Run("refresh looks anonymous subjects up as subject references", func(t *testing.T) {
		mock := &ucMockAPI{t: t, body: map[string]any{"unifiedConsent": map[string]any{"subjectId": "s"}}}
		api := httptest.NewServer(mock)
		defer api.Close()
		server := newUCProviderServer(t, api.URL)
		state := baseInputs().Set("consentId", property.New("consent-1")).Set("lastSynced", property.New(""))
		resp, err := server.Read(p.ReadRequest{
			ID: "consent-1", Urn: cmpURN("Consent", "c"), Properties: state, Inputs: baseInputs(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.ID != "consent-1" {
			t.Fatalf("expected the consent to stay tracked, got ID %q", resp.ID)
		}
		requests := mock.recorded()
		if len(requests) != 1 || requests[0].Path != "/v2/consents/unified/anon-1" || requests[0].Query["ref"] != "subject" {
			t.Fatalf("unexpected refresh request: %#v", requests)
		}
	})

	t.Run("check rejects values outside the spec", func(t *testing.T) {
		server := newUCProviderServer(t, "https://uc.invalid")
		cases := map[string]property.Map{
			"actions": baseInputs().Set("actions", property.New([]property.Value{
				property.New(map[string]property.Value{
					"target": property.New("t"), "vendor": property.New("v"), "action": property.New("accept"),
				}),
			})),
			"origin":     baseInputs().Set("origin", property.New("web")),
			"compliance": baseInputs().Set("compliance", property.New(map[string]property.Value{"gpc": property.New(2.0)})),
			"subject": baseInputs().Set("subject", property.New(map[string]property.Value{
				"verifiedId": property.New("has space"),
			})),
		}
		for property, inputs := range cases {
			resp, err := server.Check(p.CheckRequest{Urn: cmpURN("Consent", "c"), Inputs: inputs})
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Failures) != 1 || resp.Failures[0].Property != property {
				t.Fatalf("%s: expected one failure for %s, got %#v", property, property, resp.Failures)
			}
		}
	})

	t.Run("check allows a GPC consent without actions", func(t *testing.T) {
		server := newUCProviderServer(t, "https://uc.invalid")
		inputs := baseInputs().Delete("actions").Set("origin", property.New("gpc"))
		resp, err := server.Check(p.CheckRequest{Urn: cmpURN("Consent", "c"), Inputs: inputs})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Failures) != 0 {
			t.Fatalf("unexpected failures: %#v", resp.Failures)
		}
	})
}
