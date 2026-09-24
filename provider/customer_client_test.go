//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"strings"
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
