//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const (
	defaultCustomerBaseURL    = "https://api.osano.com"
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
	UCAPIKey              string `pulumi:"ucApiKey,optional" provider:"secret"`
	CustomerBaseURL       string `pulumi:"customerBaseUrl,optional"`
	APIBaseURL            string `pulumi:"apiBaseUrl,optional"`
	UCBaseURL             string `pulumi:"ucBaseUrl,optional"`
	RequestTimeoutSeconds int    `pulumi:"requestTimeoutSeconds,optional"`
}

// Annotate documents the configuration schema exposed to Pulumi users.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(
		&c.OsanoAPIKey,
		"Osano API key used for subject send-code/verify routes and Customer REST API/CMP operations "+
			"(set via pulumi config set osano:osanoApiKey --secret, or OSANO_API_KEY, which takes precedence).",
	)
	a.Describe(
		&c.UnifiedConsentAPIKey,
		"Unified Consent API key used for consent collection routes "+
			"(set via pulumi config set osano:unifiedConsentApiKey --secret, or OSANO_UC_API_KEY, "+
			"which takes precedence).",
	)
	a.Describe(&c.UCAPIKey, "Unified Consent API key for the Unified Consent Core API (x-uc-api-key).")
	a.Deprecate(&c.UCAPIKey, "use unifiedConsentApiKey instead")
	a.Describe(
		&c.CustomerBaseURL,
		"Override base URL for the Customer REST API (default: https://api.osano.com).",
	)
	a.Describe(
		&c.APIBaseURL,
		"Base URL for the Osano Unified Consent API. Override only when targeting a custom domain "+
			"(default https://uc.api.osano.com). OSANO_API_BASE_URL takes precedence when set.",
	)
	a.Describe(&c.UCBaseURL, "Override base URL for the Unified Consent Core API (default: https://uc.api.osano.com).")
	a.Deprecate(&c.UCBaseURL, "use apiBaseUrl instead")
	a.Describe(&c.RequestTimeoutSeconds,
		"HTTP request timeout in seconds for Customer REST API and Unified Consent calls (default 60). "+
			"OSANO_API_TIMEOUT_SECONDS takes precedence when set to a positive integer.",
	)
}

// Configure ensures sane defaults for optional configuration values.
func (c *Config) Configure(ctx context.Context) error {
	_ = ctx

	if c.CustomerBaseURL == "" {
		c.CustomerBaseURL = defaultCustomerBaseURL
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = c.unifiedConsentBaseURL()
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = defaultRequestTimeoutSecs
	}
	return nil
}

// providerConfigKeys lists the provider inputs that diffProviderConfig compares. Engine-managed keys
// such as version, pluginDownloadURL, and the __internal map are deliberately absent.
var providerConfigKeys = []string{
	"osanoApiKey", "unifiedConsentApiKey", "ucApiKey",
	"customerBaseUrl", "apiBaseUrl", "ucBaseUrl", "requestTimeoutSeconds",
}

// diffProviderConfig reports provider configuration changes as in-place updates, never replacements.
//
// The framework default marks every changed config key as a replacement, and the Pulumi engine then
// replaces every resource that uses the provider. Credentials, endpoints, and timeouts do not change
// the identity of any Osano resource, and Osano cannot delete Cookie Consent configs, so rotating an
// API key or tuning a timeout must not recreate configs, rules, or publications.
//
// Values are compared by their string form so the unchecked inputs recorded by `pulumi import`
// (missing keys, numbers still encoded as strings) compare equal to the checked inputs a program run
// records; otherwise the first `pulumi up` after an import would replace every imported resource.
func diffProviderConfig(_ context.Context, req p.DiffRequest) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	for _, key := range providerConfigKeys {
		if configValueString(req.State.Get(key)) != configValueString(req.Inputs.Get(key)) {
			diff[key] = p.PropertyDiff{Kind: p.Update, InputDiff: true}
		}
	}
	return p.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func configValueString(value property.Value) string {
	switch {
	case value.IsComputed():
		return "<unknown>"
	case value.IsString():
		return value.AsString()
	case value.IsNumber():
		// The only numeric input is requestTimeoutSeconds, where 0 and unset both mean the default.
		if number := value.AsNumber(); number != 0 {
			return strconv.FormatFloat(number, 'f', -1, 64)
		}
		return ""
	case value.IsBool():
		return strconv.FormatBool(value.AsBool())
	default:
		return ""
	}
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
	if baseURL == "" && cfg != nil {
		baseURL = cfg.unifiedConsentBaseURL()
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
		ucKey = cfg.unifiedConsentAPIKey()
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

func (c *Config) unifiedConsentAPIKey() string {
	return getFirstNonEmpty(c.UnifiedConsentAPIKey, c.UCAPIKey)
}

func (c *Config) unifiedConsentBaseURL() string {
	return getFirstNonEmpty(c.APIBaseURL, c.UCBaseURL, defaultAPIBaseURL)
}

func (c *Config) customerBaseURL() string {
	return getFirstNonEmpty(c.CustomerBaseURL, defaultCustomerBaseURL)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	return client
}
