//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	osanoclient "github.com/jflavan/pulumi-osano/provider/internal/osano"
)

type customerSettings struct {
	baseURL *url.URL
	apiKey  string
	timeout time.Duration
}

func customerSettingsFromConfig(cfg Config) (customerSettings, error) {
	apiKey := getFirstNonEmpty(os.Getenv(envOsanoAPIKey), cfg.OsanoAPIKey)
	if apiKey == "" {
		return customerSettings{}, errors.New(
			"Osano API key not configured; set osano:osanoApiKey or OSANO_API_KEY",
		)
	}

	baseURL, err := url.Parse(cfg.customerBaseURL())
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return customerSettings{}, fmt.Errorf("invalid customerBaseUrl %q", cfg.customerBaseURL())
	}

	timeoutSeconds := cfg.RequestTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultRequestTimeoutSecs
	}
	if value := strings.TrimSpace(os.Getenv(envRequestTimeout)); value != "" {
		if parsed, parseErr := strconv.Atoi(value); parseErr == nil && parsed > 0 {
			timeoutSeconds = parsed
		}
	}

	return customerSettings{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		timeout: time.Duration(timeoutSeconds) * time.Second,
	}, nil
}

func customerClientFromConfig(cfg Config) (*osanoclient.Client, error) {
	settings, err := customerSettingsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return osanoclient.NewClient(
		settings.baseURL,
		"x-osano-api-key",
		settings.apiKey,
		osanoclient.WithHTTPClient(newHTTPClient(settings.timeout)),
	), nil
}
