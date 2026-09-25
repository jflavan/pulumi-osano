//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCustomerSettingsFromConfig(t *testing.T) {
	t.Setenv(envOsanoAPIKey, "environment-key")
	t.Setenv(envRequestTimeout, "7")
	settings, err := customerSettingsFromConfig(Config{
		OsanoAPIKey:           "config-key",
		CustomerBaseURL:       "https://customer.example.test/root",
		RequestTimeoutSeconds: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.apiKey != "environment-key" {
		t.Fatalf("expected environment key, got %q", settings.apiKey)
	}
	if settings.timeout != 7*time.Second {
		t.Fatalf("expected 7s timeout, got %s", settings.timeout)
	}
	if settings.baseURL.String() != "https://customer.example.test/root" {
		t.Fatalf("unexpected base URL %s", settings.baseURL)
	}
}

// The Pulumi Registry asks providers to identify themselves to the vendor API, so Customer REST API
// calls send the same pulumi-osano/<version> user agent as Unified Consent calls.
func TestCustomerClientSendsProviderUserAgent(t *testing.T) {
	var got atomic.Value
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer api.Close()
	t.Setenv(envOsanoAPIKey, "")
	t.Setenv(envRequestTimeout, "")

	client, err := customerClientFromConfig(Config{OsanoAPIKey: "config-key", CustomerBaseURL: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DoJSON(t.Context(), http.MethodGet, "/v1/ping", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if ua, _ := got.Load().(string); ua != providerUserAgent() || !strings.HasPrefix(ua, "pulumi-osano/") {
		t.Fatalf("expected the provider user agent %q, got %q", providerUserAgent(), ua)
	}
}

// Released binaries are stamped with the git tag (v0.1.0) and Makefile builds with the bare version
// (0.1.0); both must send pulumi-osano/0.1.0.
func TestUserAgentForVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version string
		want    string
	}{
		{version: "0.1.0", want: "pulumi-osano/0.1.0"},
		{version: "v0.1.0", want: "pulumi-osano/0.1.0"},
		{version: "v0.1.0-alpha.1727200000", want: "pulumi-osano/0.1.0-alpha.1727200000"},
		{version: "0.1.0-alpha.0+dev", want: "pulumi-osano/0.1.0-alpha.0+dev"},
		{version: " v1.2.3 ", want: "pulumi-osano/1.2.3"},
		{version: "", want: "pulumi-osano/dev"},
		{version: "  ", want: "pulumi-osano/dev"},
	}
	for _, tt := range tests {
		if got := userAgentForVersion(tt.version); got != tt.want {
			t.Errorf("userAgentForVersion(%q) = %q, want %q", tt.version, got, tt.want)
		}
	}
}

func TestCustomerSettingsFromConfigWhitespaceEnvironmentFallsBackToConfig(t *testing.T) {
	t.Setenv(envOsanoAPIKey, " \t ")
	t.Setenv(envRequestTimeout, " \t ")

	settings, err := customerSettingsFromConfig(Config{
		OsanoAPIKey:           "config-key",
		RequestTimeoutSeconds: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.apiKey != "config-key" {
		t.Fatalf("expected config key, got %q", settings.apiKey)
	}
	if settings.timeout != 9*time.Second {
		t.Fatalf("expected 9s timeout, got %s", settings.timeout)
	}
}

func TestCustomerSettingsFromConfigInvalidEnvironmentTimeoutFallsBack(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment string
		configured  int
		want        time.Duration
	}{
		{name: "zero uses config", environment: "0", configured: 9, want: 9 * time.Second},
		{name: "negative uses config", environment: "-1", configured: 9, want: 9 * time.Second},
		{name: "invalid uses config", environment: "invalid", configured: 9, want: 9 * time.Second},
		{name: "zero uses default", environment: "0", configured: 0, want: 60 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(envRequestTimeout, test.environment)
			settings, err := customerSettingsFromConfig(Config{
				OsanoAPIKey:           "config-key",
				RequestTimeoutSeconds: test.configured,
			})
			if err != nil {
				t.Fatal(err)
			}
			if settings.timeout != test.want {
				t.Fatalf("expected timeout %s, got %s", test.want, settings.timeout)
			}
		})
	}
}

func TestCustomerSettingsFromConfigRejectsMalformedBaseURL(t *testing.T) {
	settings, err := customerSettingsFromConfig(Config{
		OsanoAPIKey:     "config-key",
		CustomerBaseURL: "not a URL",
	})
	if err == nil {
		t.Fatalf("expected malformed URL error, got settings %#v", settings)
	}
	if !strings.Contains(err.Error(), `invalid customerBaseUrl "not a URL"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCustomerSettingsFromConfigReportsMissingAPIKey(t *testing.T) {
	t.Setenv(envOsanoAPIKey, "")

	_, err := customerSettingsFromConfig(Config{})
	if err == nil {
		t.Fatal("expected missing API key error")
	}
	if err.Error() != "Osano API key not configured; set osano:osanoApiKey or OSANO_API_KEY" {
		t.Fatalf("unexpected error: %v", err)
	}
}
