package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Osano's GET reports configuration and palette keys set in the Osano dashboard that its spec does
// not define, while its PATCH declares both objects with additionalProperties: false and rejects
// any request naming one, even to null it (issue #33). These tests cover how the provider keeps such
// keys out of what it imports and sends.

// dashboardConfiguration is a configuration as Osano's GET reports it: spec keys plus keys only the
// dashboard sets, at the top level and in palette.
func dashboardConfiguration() map[string]any {
	return map[string]any{
		"storagePolicyHref":    "https://example.com/storage-policy",
		"showWidget":           true,
		"analyticsOnByDefault": false,
		"iabEnabled":           false,
		"iframes":              map[string]any{"youtube": true},
		"palette": map[string]any{
			"linkColor":   "#111111",
			"widgetColor": "#222222",
			"borderless":  false,
		},
		"translations": map[string]any{"en": map[string]any{"anyKey": "kept"}},
	}
}

// modelledDashboardConfiguration is dashboardConfiguration without the keys Osano's spec omits.
func modelledDashboardConfiguration() map[string]any {
	return map[string]any{
		"storagePolicyHref": "https://example.com/storage-policy",
		"showWidget":        true,
		"palette":           map[string]any{"linkColor": "#111111"},
		"translations":      map[string]any{"en": map[string]any{"anyKey": "kept"}},
	}
}

func TestWithoutUnmodelledKeys(t *testing.T) {
	t.Parallel()

	t.Run("leaves out top-level and palette keys outside the spec", func(t *testing.T) {
		configuration := dashboardConfiguration()
		if got := withoutUnmodelledKeys(configuration); !reflect.DeepEqual(got, modelledDashboardConfiguration()) {
			t.Fatalf("unexpected configuration: %#v", got)
		}
		if !reflect.DeepEqual(configuration, dashboardConfiguration()) {
			t.Fatalf("the configuration must not be modified, got %#v", configuration)
		}
	})

	t.Run("keeps nulls and an explicit empty palette", func(t *testing.T) {
		configuration := map[string]any{
			"storagePolicyHref": "https://example.com/storage-policy",
			"googleConsent":     nil,
			"palette":           map[string]any{},
		}
		if got := withoutUnmodelledKeys(configuration); !reflect.DeepEqual(got, configuration) {
			t.Fatalf("unexpected configuration: %#v", got)
		}
		configuration["palette"] = nil
		if got := withoutUnmodelledKeys(configuration); !reflect.DeepEqual(got, configuration) {
			t.Fatalf("unexpected configuration: %#v", got)
		}
	})

	t.Run("leaves out a palette that held only unmodelled keys", func(t *testing.T) {
		got := withoutUnmodelledKeys(map[string]any{
			"storagePolicyHref": "https://example.com/storage-policy",
			"palette":           map[string]any{"widgetColor": nil},
		})
		if _, present := got["palette"]; present {
			t.Fatalf("expected palette to be left out rather than sent as {}, got %#v", got)
		}
	})

	t.Run("nil stays nil", func(t *testing.T) {
		if got := withoutUnmodelledKeys(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})
}

func TestCookieConsentConfigImportLeavesOutUnmodelledKeys(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCMPRequest(t, r, http.MethodGet, "/v1/cookie-consent/configs/config-123")
		fixture := cmpConfigResponseFixture()
		fixture.Configuration = dashboardConfiguration()
		writeJSON(t, w, fixture)
	}))
	defer api.Close()

	resp, err := newCMPProviderServer(t, api.URL).Read(p.ReadRequest{
		ID:         "config-123",
		Urn:        cmpURN("CookieConsentConfig", "imported"),
		Properties: configStateProperties(),
		Inputs:     emptyConfigInputProperties(),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := toPropertyValue(modelledDashboardConfiguration())
	for _, properties := range []property.Map{resp.Inputs, resp.Properties} {
		if got := properties.Get("configuration"); !got.Equals(want) {
			t.Fatalf("expected the import to adopt only spec keys:\n got: %v\nwant: %v", got, want)
		}
	}
}

// A refresh projects Osano's configuration onto the keys the previous inputs declare, whatever they
// are, so a program that still declares an unmodelled key does not diff on every refresh.
func TestCookieConsentConfigRefreshKeepsDeclaredUnmodelledKeys(t *testing.T) {
	t.Parallel()

	resp := cmpConfigResponseFixture()
	resp.Configuration = dashboardConfiguration()
	declared := baseConfigArgs()
	declared.Configuration = dashboardConfiguration()
	args := cookieConsentConfigArgsFromResponse(resp, declared)
	if !reflect.DeepEqual(args.Configuration, dashboardConfiguration()) {
		t.Fatalf("expected the declared keys to be kept, got %#v", args.Configuration)
	}
}

func TestCookieConsentConfigCreateLeavesOutUnmodelledKeys(t *testing.T) {
	var body atomic.Value
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCMPRequest(t, r, http.MethodPost, "/v1/cookie-consent/configs")
		body.Store(decodeJSONBody(t, r))
		writeCMPConfigResponse(t, w)
	}))
	defer api.Close()

	inputs := configInputProperties().Set("configuration", toPropertyValue(dashboardConfiguration()))
	resp, err := newCMPProviderServer(t, api.URL).Create(p.CreateRequest{
		Urn: cmpURN("CookieConsentConfig", "created"), Properties: inputs,
	})
	if err != nil {
		t.Fatal(err)
	}
	sent, _ := body.Load().(map[string]any)
	if !reflect.DeepEqual(sent["configuration"], modelledDashboardConfiguration()) {
		t.Fatalf("expected only spec keys to be sent, got %#v", sent["configuration"])
	}
	if got := resp.Properties.Get("configuration"); !got.Equals(inputs.Get("configuration")) {
		t.Fatalf("state must record the declared configuration, got %v", got)
	}
}

// A stack imported before unmodelled keys were left out declares them in the program and holds them
// in state. Each way out of that has to work without editing the state.
func TestCookieConsentConfigUpdateWithUnmodelledKeys(t *testing.T) {
	imported := configStateProperties().Set("configuration", toPropertyValue(dashboardConfiguration()))

	t.Run("a change keeps declaring them", func(t *testing.T) {
		var body atomic.Value
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertCMPRequest(t, r, http.MethodPatch, "/v1/cookie-consent/configs/config-123")
			body.Store(decodeJSONBody(t, r))
			writeCMPConfigResponse(t, w)
		}))
		defer api.Close()

		configuration := dashboardConfiguration()
		configuration["showWidget"] = false
		inputs := configInputProperties().Set("configuration", toPropertyValue(configuration))
		resp, err := newCMPProviderServer(t, api.URL).Update(p.UpdateRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "imported"), State: imported, Inputs: inputs,
		})
		if err != nil {
			t.Fatal(err)
		}
		want := modelledDashboardConfiguration()
		want["showWidget"] = false
		sent, _ := body.Load().(map[string]any)
		if !reflect.DeepEqual(sent["configuration"], want) {
			t.Fatalf("expected only spec keys to be sent:\n got: %#v\nwant: %#v", sent["configuration"], want)
		}
		if got := resp.Properties.Get("configuration"); !got.Equals(inputs.Get("configuration")) {
			t.Fatalf("state must record the declared configuration, got %v", got)
		}
	})

	t.Run("removing only them sends nothing", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("an update that changes only unmodelled keys must not call Osano: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer api.Close()

		inputs := configInputProperties().Set("configuration", toPropertyValue(modelledDashboardConfiguration()))
		server := newCMPProviderServer(t, api.URL)
		diff, err := server.Diff(p.DiffRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "imported"), State: imported, Inputs: inputs,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !diff.HasChanges {
			t.Fatal("removing keys from the program must still update the state")
		}
		resp, err := server.Update(p.UpdateRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "imported"), State: imported, Inputs: inputs,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Properties.Get("configuration"); !got.Equals(inputs.Get("configuration")) {
			t.Fatalf("state must drop the removed keys, got %v", got)
		}
		assertKnownString(t, resp.Properties, "configId", "config-123")
	})

	t.Run("removing them with another change nulls only spec keys", func(t *testing.T) {
		var body atomic.Value
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertCMPRequest(t, r, http.MethodPatch, "/v1/cookie-consent/configs/config-123")
			body.Store(decodeJSONBody(t, r))
			writeCMPConfigResponse(t, w)
		}))
		defer api.Close()

		configuration := modelledDashboardConfiguration()
		delete(configuration, "showWidget")
		inputs := configInputProperties().Set("configuration", toPropertyValue(configuration))
		if _, err := newCMPProviderServer(t, api.URL).Update(p.UpdateRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "imported"), State: imported, Inputs: inputs,
		}); err != nil {
			t.Fatal(err)
		}
		want := modelledDashboardConfiguration()
		want["showWidget"] = nil
		sent, _ := body.Load().(map[string]any)
		if !reflect.DeepEqual(sent["configuration"], want) {
			t.Fatalf("expected a null for the removed spec key only:\n got: %#v\nwant: %#v", sent["configuration"], want)
		}
	})
}

// The decision to skip the request follows what the PATCH would send, nulls for removed keys
// included, not the declared configuration alone.
func TestCookieConsentConfigUpdateWithDashboardOnlyPalette(t *testing.T) {
	previous := configStateProperties().Set("configuration", toPropertyValue(map[string]any{
		"storagePolicyHref": "https://example.com/storage-policy",
		"palette":           map[string]any{"widgetColor": "#222222"},
	}))

	// Removing the palette sends palette: null, which Osano accepts, so the request is still made.
	t.Run("removing the palette clears it", func(t *testing.T) {
		var body atomic.Value
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertCMPRequest(t, r, http.MethodPatch, "/v1/cookie-consent/configs/config-123")
			body.Store(decodeJSONBody(t, r))
			writeCMPConfigResponse(t, w)
		}))
		defer api.Close()

		inputs := configInputProperties().Set("configuration", toPropertyValue(map[string]any{
			"storagePolicyHref": "https://example.com/storage-policy",
		}))
		if _, err := newCMPProviderServer(t, api.URL).Update(p.UpdateRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "palette"), State: previous, Inputs: inputs,
		}); err != nil {
			t.Fatal(err)
		}
		sent, _ := body.Load().(map[string]any)
		configuration, _ := sent["configuration"].(map[string]any)
		if value, present := configuration["palette"]; !present || value != nil {
			t.Fatalf("expected palette to be sent as null, got %#v", configuration)
		}
	})

	// Emptying the palette would send nothing new: the unmodelled key is left out, and so is the
	// palette it leaves empty.
	t.Run("emptying the palette sends nothing", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("an update that sends nothing new must not call Osano: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer api.Close()

		inputs := configInputProperties().Set("configuration", toPropertyValue(map[string]any{
			"storagePolicyHref": "https://example.com/storage-policy",
			"palette":           map[string]any{},
		}))
		resp, err := newCMPProviderServer(t, api.URL).Update(p.UpdateRequest{
			ID: "config-123", Urn: cmpURN("CookieConsentConfig", "palette"), State: previous, Inputs: inputs,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Properties.Get("configuration"); !got.Equals(inputs.Get("configuration")) {
			t.Fatalf("state must record the declared configuration, got %v", got)
		}
	})
}

// toPropertyValue converts a decoded JSON value into a property value.
func toPropertyValue(value any) property.Value {
	switch typed := value.(type) {
	case nil:
		return property.Value{}
	case bool:
		return property.New(typed)
	case float64:
		return property.New(typed)
	case string:
		return property.New(typed)
	case []any:
		items := make([]property.Value, len(typed))
		for idx, item := range typed {
			items[idx] = toPropertyValue(item)
		}
		return property.New(items)
	case map[string]any:
		fields := make(map[string]property.Value, len(typed))
		for key, item := range typed {
			fields[key] = toPropertyValue(item)
		}
		return property.New(fields)
	default:
		panic(fmt.Sprintf("unsupported JSON value %#v", value))
	}
}
