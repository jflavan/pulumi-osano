package provider

import (
	"context"
	"net/url"
	"os"
	"testing"

	osanoclient "github.com/jflavan/pulumi-osano/provider/internal/osano"
)

// Integration tests require a live Osano API key.
// Set OSANO_API_KEY to run them. Optionally set OSANO_CONFIG_ID to test
// reading an existing config.
//
// Run: OSANO_API_KEY=<key> go test -race -v -run TestIntegration ./...

func skipUnlessIntegration(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OSANO_API_KEY")
	if key == "" {
		t.Skip("skipping integration test: OSANO_API_KEY not set")
	}
	return key
}

func integrationClient(t *testing.T) *osanoclient.Client {
	t.Helper()
	key := skipUnlessIntegration(t)
	base, _ := url.Parse("https://api.osano.com")
	return osanoclient.NewClient(base, "x-osano-api-key", key)
}

func TestIntegrationListConfigs(t *testing.T) {
	c := integrationClient(t)

	var out struct {
		Items []struct {
			ConfigId string `json:"configId"`
			Name     string `json:"name"`
		} `json:"items"`
	}
	err := c.DoJSON(context.Background(), "GET", "/v1/cookie-consent/configs", nil, nil, &out)
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}
	t.Logf("found %d configs", len(out.Items))
	for _, item := range out.Items {
		t.Logf("  %s: %s", item.ConfigId, item.Name)
	}
}

func TestIntegrationGetConfig(t *testing.T) {
	c := integrationClient(t)
	configId := os.Getenv("OSANO_CONFIG_ID")
	if configId == "" {
		t.Skip("skipping: OSANO_CONFIG_ID not set")
	}

	var out cmpConfigResponse
	err := c.DoJSON(context.Background(), "GET", "/v1/cookie-consent/configs/"+url.PathEscape(configId), nil, nil, &out)
	if err != nil {
		t.Fatalf("get config %s: %v", configId, err)
	}
	if out.ConfigId != configId {
		t.Fatalf("expected configId %q, got %q", configId, out.ConfigId)
	}
	t.Logf("config %s: name=%q mode=%q domains=%v", out.ConfigId, out.Name, out.Mode, out.Domains)
}
