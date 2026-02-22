package provider

import (
    "context"
    "fmt"
    "net/url"

    "github.com/pulumi/pulumi-go-provider/infer"
)

type Config struct {
    // Customer REST API key.
    OsanoApiKey string `pulumi:"osanoApiKey,optional" provider:"secret"`

    // Unified Consent Core API key.
    UcApiKey string `pulumi:"ucApiKey,optional" provider:"secret"`

    // Base URL for the Osano Customer REST API.
    CustomerBaseUrl string `pulumi:"customerBaseUrl,optional"`

    // Base URL for the Osano Unified Consent Core API.
    UcBaseUrl string `pulumi:"ucBaseUrl,optional"`

    // internal normalized URLs
    customerBase *url.URL
    ucBase       *url.URL
}

var _ = (infer.CustomConfigure)((*Config)(nil))
var _ = (infer.Annotated)((*Config)(nil))

func (c *Config) Annotate(a infer.Annotator) {
    a.Describe(&c.OsanoApiKey, "Osano API key for the Customer REST API (x-osano-api-key).")
    a.Describe(&c.UcApiKey, "Unified Consent API key for the Unified Consent Core API (x-uc-api-key).")
    a.Describe(&c.CustomerBaseUrl, "Override base URL for the Customer REST API (default: https://api.osano.com).")
    a.Describe(&c.UcBaseUrl, "Override base URL for the Unified Consent Core API (default: https://uc.api.osano.com).")
}

func (c *Config) Configure(ctx context.Context) error {
    if c.CustomerBaseUrl == "" {
        c.CustomerBaseUrl = "https://api.osano.com"
    }
    if c.UcBaseUrl == "" {
        c.UcBaseUrl = "https://uc.api.osano.com"
    }

    u, err := url.Parse(c.CustomerBaseUrl)
    if err != nil {
        return fmt.Errorf("invalid customerBaseUrl: %w", err)
    }
    c.customerBase = u

    u, err = url.Parse(c.UcBaseUrl)
    if err != nil {
        return fmt.Errorf("invalid ucBaseUrl: %w", err)
    }
    c.ucBase = u

    return nil
}
