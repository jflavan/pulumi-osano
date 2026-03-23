//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	defaultAPIBaseURL         = "https://uc.api.osano.com"
	defaultRequestTimeoutSecs = 60

	envOsanoAPIKey    = "OSANO_API_KEY" //nolint:gosec // Environment variable name, not a credential.
	envUnifiedConsent = "OSANO_UC_API_KEY"
	envAPIBaseURL     = "OSANO_API_BASE_URL"
	envRequestTimeout = "OSANO_API_TIMEOUT_SECONDS"
)

// Config defines provider-level settings for talking to the Osano API.
type Config struct {
	OsanoAPIKey           string `pulumi:"osanoApiKey,optional" provider:"secret"`
	UnifiedConsentAPIKey  string `pulumi:"unifiedConsentApiKey,optional" provider:"secret"`
	APIBaseURL            string `pulumi:"apiBaseUrl,optional"`
	RequestTimeoutSeconds int    `pulumi:"requestTimeoutSeconds,optional"`
}

// Annotate documents the configuration schema exposed to Pulumi users.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(
		&c.OsanoAPIKey,
		"Osano API key used for subject profile routes "+
			"(set via pulumi config set osano:osanoApiKey --secret or OSANO_API_KEY).",
	)
	a.Describe(
		&c.UnifiedConsentAPIKey,
		"Unified Consent API key used for consent collection routes "+
			"(set via pulumi config set osano:unifiedConsentApiKey --secret or OSANO_UC_API_KEY).",
	)
	a.Describe(
		&c.APIBaseURL,
		"Base URL for the Osano Unified Consent API. Override only when targeting a custom domain "+
			"(default https://uc.api.osano.com).",
	)
	a.Describe(&c.RequestTimeoutSeconds,
		"HTTP request timeout in seconds for Osano API calls (default 60).",
	)
}

// Configure ensures sane defaults for optional configuration values.
func (c *Config) Configure(ctx context.Context) error {
	if c.APIBaseURL == "" {
		c.APIBaseURL = defaultAPIBaseURL
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = defaultRequestTimeoutSecs
	}
	return nil
}

type apiSettings struct {
	baseURL              string
	osanoAPIKey          string
	unifiedConsentAPIKey string
	timeout              time.Duration
}

func (s *apiSettings) normalizedBaseURL() string {
	return strings.TrimRight(s.baseURL, "/")
}

func loadAPISettings(ctx context.Context) *apiSettings {
	cfg := infer.GetConfig[*Config](ctx)

	baseURL := getFirstNonEmpty(os.Getenv(envAPIBaseURL))
	if baseURL == "" && cfg != nil && cfg.APIBaseURL != "" {
		baseURL = cfg.APIBaseURL
	}
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}

	osanoKey := getFirstNonEmpty(os.Getenv(envOsanoAPIKey))
	if osanoKey == "" && cfg != nil {
		osanoKey = cfg.OsanoAPIKey
	}

	ucKey := getFirstNonEmpty(os.Getenv(envUnifiedConsent))
	if ucKey == "" && cfg != nil {
		ucKey = cfg.UnifiedConsentAPIKey
	}

	timeout := defaultRequestTimeoutSecs
	if cfg != nil && cfg.RequestTimeoutSeconds > 0 {
		timeout = cfg.RequestTimeoutSeconds
	}
	if v := strings.TrimSpace(os.Getenv(envRequestTimeout)); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	return &apiSettings{
		baseURL:              baseURL,
		osanoAPIKey:          strings.TrimSpace(osanoKey),
		unifiedConsentAPIKey: strings.TrimSpace(ucKey),
		timeout:              time.Duration(timeout) * time.Second,
	}
}

func getFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func newHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	return client
}
