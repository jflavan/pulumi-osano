package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type cmpFunctionRequest struct {
	Method   string
	Path     string
	RawQuery string
}

// cmpFunctionAPI serves canned Customer REST API responses keyed by path and records each request.
type cmpFunctionAPI struct {
	t         *testing.T
	mu        sync.Mutex
	requests  []cmpFunctionRequest
	responses map[string]func(r *http.Request) (int, any)
}

func (m *cmpFunctionAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("x-osano-api-key"); got != "test-osano-key" {
		m.t.Errorf("expected x-osano-api-key header, got %q", got)
	}
	m.mu.Lock()
	m.requests = append(m.requests, cmpFunctionRequest{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery})
	respond, ok := m.responses[r.URL.Path]
	m.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	status, body := respond(r)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		writeJSON(m.t, w, body)
	}
}

func (m *cmpFunctionAPI) recorded() []cmpFunctionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]cmpFunctionRequest(nil), m.requests...)
}

func invokeCMP(
	t *testing.T, responses map[string]func(*http.Request) (int, any), token string, args map[string]property.Value,
) (p.InvokeResponse, []cmpFunctionRequest, error) {
	t.Helper()
	api := &cmpFunctionAPI{t: t, responses: responses}
	server := httptest.NewServer(api)
	defer server.Close()
	provider := newCMPProviderServer(t, server.URL)
	resp, err := provider.Invoke(p.InvokeRequest{
		Token: tokens.Type("osano:index:" + token),
		Args:  property.NewMap(args),
	})
	return resp, api.recorded(), err
}

func respondJSON(status int, body any) func(*http.Request) (int, any) {
	return func(*http.Request) (int, any) { return status, body }
}

func TestGetCookieConsentConfig(t *testing.T) {
	t.Run("returns the configuration and its install script", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs/config-123": respondJSON(http.StatusOK, cmpConfigResponseFixture()),
		}, "getCookieConsentConfig", map[string]property.Value{"configId": property.New(" config-123 ")})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || requests[0].Method != http.MethodGet {
			t.Fatalf("unexpected requests: %#v", requests)
		}
		ret := resp.Return
		assertBool(t, ret, "exists", true)
		assertString(t, ret, "configId", "config-123")
		assertString(t, ret, "publishStatus", "published")
		assertString(t, ret, "scriptSrc", "https://cmp.osano.com/customer-123/config-123/osano.js")
		assertString(t, ret, "scriptTag",
			`<script src="https://cmp.osano.com/customer-123/config-123/osano.js"></script>`)
		if got := ret.Get("publishedRevision").AsNumber(); got != 7 {
			t.Fatalf("expected publishedRevision 7, got %v", got)
		}
		if !ret.Get("configuration").AsMap().Get("flag").AsBool() {
			t.Fatalf("expected the full configuration, got %#v", ret.Get("configuration"))
		}
	})

	t.Run("maps 404 to a missing configuration", func(t *testing.T) {
		resp, _, err := invokeCMP(t, map[string]func(*http.Request) (int, any){}, "getCookieConsentConfig",
			map[string]property.Value{"configId": property.New("missing")})
		if err != nil {
			t.Fatal(err)
		}
		assertBool(t, resp.Return, "exists", false)
		assertString(t, resp.Return, "scriptTag", "")
	})
}

func TestGetCookieConsentConfigs(t *testing.T) {
	page := func(ids []string, next string) map[string]any {
		items := make([]any, 0, len(ids))
		for _, id := range ids {
			fixture := cmpConfigResponseFixture()
			fixture.ConfigID = id
			items = append(items, fixture)
		}
		return map[string]any{"items": items, "next": next}
	}

	t.Run("sends every filter and follows pagination", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs": func(r *http.Request) (int, any) {
				if r.URL.Query().Get("next") == "" {
					return http.StatusOK, page([]string{"a", "b"}, "cursor-1")
				}
				return http.StatusOK, page([]string{"c"}, "")
			},
		}, "getCookieConsentConfigs", map[string]property.Value{
			"name":                property.New("marketing site"),
			"domains":             property.New([]property.Value{property.New("example.com"), property.New("example.org")}),
			"domainsMatch":        property.New("all"),
			"orgIds":              property.New([]property.Value{property.New("11111111-1111-1111-1111-111111111111")}),
			"mode":                property.New("production"),
			"publishStatus":       property.New("outdated"),
			"tattleRecordStopped": property.New(false),
			"sortBy":              property.New("updated"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 2 {
			t.Fatalf("expected two pages, got %#v", requests)
		}
		first := requests[0].RawQuery
		for _, want := range []string{
			"name=marketing%20site", "domains=all%3Aexample.com%2Cexample.org",
			"orgIds=11111111-1111-1111-1111-111111111111", "mode=production", "status=outdated",
			"tattleRecordStopped=false", "sortBy=updated", "limit=1000",
		} {
			if !strings.Contains(first, want) {
				t.Fatalf("expected query to contain %q, got %q", want, first)
			}
		}
		if second := requests[1].RawQuery; !strings.Contains(second, "next=cursor-1") ||
			!strings.Contains(second, "mode=production") {
			t.Fatalf("expected the next page to keep the filters and add next, got %q", second)
		}
		configs := resp.Return.Get("configs").AsArray()
		if configs.Len() != 3 {
			t.Fatalf("expected three configs, got %d", configs.Len())
		}
		assertString(t, configs.Get(2).AsMap(), "scriptTag",
			`<script src="https://cmp.osano.com/customer-123/c/osano.js"></script>`)
	})

	t.Run("maxResults stops early and shrinks the page size", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs": respondJSON(http.StatusOK, page([]string{"a", "b"}, "cursor-1")),
		}, "getCookieConsentConfigs", map[string]property.Value{"maxResults": property.New(1.0)})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || !strings.Contains(requests[0].RawQuery, "limit=1") {
			t.Fatalf("expected one request with limit=1, got %#v", requests)
		}
		if got := resp.Return.Get("configs").AsArray().Len(); got != 1 {
			t.Fatalf("expected one config, got %d", got)
		}
	})

	for name, args := range map[string]map[string]property.Value{
		"unknown mode":         {"mode": property.New("strict")},
		"unknown sort field":   {"sortBy": property.New("revision")},
		"match without values": {"domainsMatch": property.New("not")},
		"unknown match": {
			"domains":      property.New([]property.Value{property.New("a.com")}),
			"domainsMatch": property.New("none"),
		},
		"comma inside a domain":   {"domains": property.New([]property.Value{property.New("a.com,b.com")})},
		"negative maximum result": {"maxResults": property.New(-1.0)},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){}, "getCookieConsentConfigs", args)
			if err == nil && len(resp.Failures) == 0 {
				t.Fatal("expected the invoke to fail")
			}
			if len(requests) != 0 {
				t.Fatalf("expected no request, got %#v", requests)
			}
		})
	}
}

func TestGetCookieConsentRules(t *testing.T) {
	t.Run("maps store types and follows pagination", func(t *testing.T) {
		title := "Google Analytics"
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/configs/config-123/rules": func(r *http.Request) (int, any) {
				if r.URL.Query().Get("next") == "" {
					return http.StatusOK, map[string]any{
						"items": []any{cmpRuleResponse{
							Type: "cookie", RuleID: 7, Classification: "ANALYTICS", Rule: "_ga", Title: &title,
							VendorID: "vendor-1", Created: "2026-01-01T00:00:00Z",
						}},
						"next": "cursor-1",
					}
				}
				return http.StatusOK, map[string]any{"items": []any{cmpRuleResponse{
					Type: "iframe", RuleID: 8, Classification: "ANALYTICS", Rule: "youtube.com",
				}}}
			},
		}, "getCookieConsentRules", map[string]property.Value{
			"configId":       property.New("config-123"),
			"storeType":      property.New("cookies"),
			"classification": property.New("ANALYTICS"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 2 || !strings.Contains(requests[0].RawQuery, "type=cookie") ||
			!strings.Contains(requests[0].RawQuery, "classification=ANALYTICS") ||
			!strings.Contains(requests[1].RawQuery, "next=cursor-1") {
			t.Fatalf("unexpected requests: %#v", requests)
		}
		rules := resp.Return.Get("rules").AsArray()
		if rules.Len() != 2 {
			t.Fatalf("expected two rules, got %d", rules.Len())
		}
		first := rules.Get(0).AsMap()
		assertString(t, first, "storeType", "cookies")
		assertString(t, first, "configId", "config-123")
		assertString(t, first, "title", "Google Analytics")
		assertString(t, first, "vendorId", "vendor-1")
		assertString(t, rules.Get(1).AsMap(), "storeType", "iframes")
	})

	t.Run("rejects an unknown store type", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){}, "getCookieConsentRules",
			map[string]property.Value{"configId": property.New("c"), "storeType": property.New("cookie")})
		if err == nil && len(resp.Failures) == 0 {
			t.Fatal("expected the invoke to fail")
		}
		if len(requests) != 0 {
			t.Fatalf("expected no request, got %#v", requests)
		}
	})
}

func TestGetCookieConsentDiscoveries(t *testing.T) {
	for _, tc := range []struct {
		storeType, wantType string
	}{{"", "cookie"}, {"scripts", "script"}, {"localStorage", "localStorage"}} {
		t.Run(fmt.Sprintf("storeType %q", tc.storeType), func(t *testing.T) {
			args := map[string]property.Value{"configId": property.New("config-123")}
			if tc.storeType != "" {
				args["storeType"] = property.New(tc.storeType)
			}
			resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
				"/v1/cookie-consent/configs/config-123/discoveries": respondJSON(http.StatusOK, map[string]any{
					"items": []any{map[string]any{
						"storeKey": "_hjid", "storeType": tc.wantType, "created": "2026-01-01T00:00:00Z",
						"updated": "2026-01-02T00:00:00Z", "scanOrigin": "osano.js", "firstPageSeen": "https://example.com/",
						"confidence": "High",
					}},
				}),
			}, "getCookieConsentDiscoveries", args)
			if err != nil {
				t.Fatal(err)
			}
			if len(requests) != 1 || requests[0].RawQuery != "type="+tc.wantType {
				t.Fatalf("unexpected requests: %#v", requests)
			}
			discoveries := resp.Return.Get("discoveries").AsArray()
			if discoveries.Len() != 1 {
				t.Fatalf("expected one discovery, got %d", discoveries.Len())
			}
			assertString(t, discoveries.Get(0).AsMap(), "confidence", "High")
			assertString(t, discoveries.Get(0).AsMap(), "storeKey", "_hjid")
		})
	}
}

func TestGetCookieConsentAuditLog(t *testing.T) {
	event := func(id string) map[string]any {
		return map[string]any{
			"id": id, "module": "CMP", "eventType": "cmp.configPublished", "actor": "ops@example.com",
			"timestamp": "2026-09-01T00:00:00Z", "metadata": map[string]any{"revision": 8},
			"resources": []any{map[string]any{"resourceId": "config-123", "resourceType": "CMP", "isPrimary": true}},
		}
	}

	t.Run("sends filters, then only the next token", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/audit-log": func(r *http.Request) (int, any) {
				if r.URL.Query().Get("next") == "" {
					return http.StatusOK, map[string]any{"items": []any{event("e1")}, "next": "cursor-1"}
				}
				return http.StatusOK, map[string]any{"items": []any{event("e2")}}
			},
		}, "getCookieConsentAuditLog", map[string]property.Value{
			"configIds":  property.New([]property.Value{property.New("config-123"), property.New("config-456")}),
			"eventTypes": property.New([]property.Value{property.New("cmp.configPublished")}),
			"changeType": property.New("rule"),
			"startDate":  property.New("2026-09-01T00:00:00Z"),
			"maxResults": property.New(0.0),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 2 {
			t.Fatalf("expected two pages, got %#v", requests)
		}
		for _, want := range []string{
			"configIds=config-123%2Cconfig-456", "eventTypes=cmp.configPublished", "changeType=rule", "limit=200",
			"startDate=2026-09-01T00%3A00%3A00Z",
		} {
			if !strings.Contains(requests[0].RawQuery, want) {
				t.Fatalf("expected query to contain %q, got %q", want, requests[0].RawQuery)
			}
		}
		if requests[1].RawQuery != "next=cursor-1" {
			t.Fatalf("expected the next page to send only the token, got %q", requests[1].RawQuery)
		}
		events := resp.Return.Get("events").AsArray()
		if events.Len() != 2 {
			t.Fatalf("expected two events, got %d", events.Len())
		}
		first := events.Get(0).AsMap()
		assertString(t, first, "eventType", "cmp.configPublished")
		assertString(t, first, "actor", "ops@example.com")
		assertString(t, first.Get("resources").AsArray().Get(0).AsMap(), "resourceId", "config-123")
	})

	t.Run("defaults to the 200 most recent events", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){
			"/v1/cookie-consent/audit-log": respondJSON(http.StatusOK, map[string]any{
				"items": []any{event("e1")}, "next": "cursor-1",
			}),
		}, "getCookieConsentAuditLog", map[string]property.Value{"maxResults": property.New(1.0)})
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 || !strings.Contains(requests[0].RawQuery, "limit=1") {
			t.Fatalf("unexpected requests: %#v", requests)
		}
		if got := resp.Return.Get("events").AsArray().Len(); got != 1 {
			t.Fatalf("expected one event, got %d", got)
		}
	})

	t.Run("rejects an unknown change type", func(t *testing.T) {
		resp, requests, err := invokeCMP(t, map[string]func(*http.Request) (int, any){}, "getCookieConsentAuditLog",
			map[string]property.Value{"changeType": property.New("colors")})
		if err == nil && len(resp.Failures) == 0 {
			t.Fatal("expected the invoke to fail")
		}
		if len(requests) != 0 {
			t.Fatalf("expected no request, got %#v", requests)
		}
	})
}

func TestCookieConsentFunctionsRequireAnAPIKey(t *testing.T) {
	for _, env := range []string{envOsanoAPIKey, envUnifiedConsent, envAPIBaseURL, envRequestTimeout} {
		t.Setenv(env, "")
	}
	server := newUnconfiguredProviderServer(t)
	if err := server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{})}); err != nil {
		t.Fatal(err)
	}
	resp, err := server.Invoke(p.InvokeRequest{
		Token: tokens.Type("osano:index:" + "getCookieConsentConfig"),
		Args:  property.NewMap(map[string]property.Value{"configId": property.New("config-123")}),
	})
	if err == nil && len(resp.Failures) == 0 {
		t.Fatal("expected a missing API key to fail the invoke")
	}
	if err != nil && !strings.Contains(err.Error(), "Osano API key not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
