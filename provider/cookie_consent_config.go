package provider

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/url"
    "slices"

    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"

    osanoclient "github.com/highfiveghost/pulumi-osano/provider/internal/osano"
)

// CookieConsentConfig manages a Cookie Consent (CMP) configuration via the Osano Customer REST API.
//
// API: /v1/cookie-consent/configs
// Spec: https://developers.osano.com/redocusaurus/customer-rest-api.yaml
//
// Note: Osano's API (per the OpenAPI spec) does not expose a delete endpoint for configs.
// Delete() is implemented as a no-op, which will orphan the remote config.
// This matches the common "retain" behavior some APIs force on us, but it should be documented.
type CookieConsentConfig struct{}

var _ = (infer.CustomCheck[CookieConsentConfigArgs])((*CookieConsentConfig)(nil))
var _ = (infer.CustomDiff[CookieConsentConfigArgs, CookieConsentConfigState])((*CookieConsentConfig)(nil))
var _ = (infer.CustomRead[CookieConsentConfigArgs, CookieConsentConfigState])((*CookieConsentConfig)(nil))
var _ = (infer.CustomUpdate[CookieConsentConfigArgs, CookieConsentConfigState])((*CookieConsentConfig)(nil))
var _ = (infer.CustomDelete[CookieConsentConfigState])((*CookieConsentConfig)(nil))
var _ = (infer.Annotated)((*CookieConsentConfig)(nil))
var _ = (infer.Annotated)((*CookieConsentConfigArgs)(nil))
var _ = (infer.Annotated)((*CookieConsentConfigState)(nil))

func (r *CookieConsentConfig) Annotate(a infer.Annotator) {
    a.Describe(&r, "Manages an Osano Cookie Consent (CMP) configuration.")
}

type CookieConsentConfigArgs struct {
    Name          string         `pulumi:"name"`
    Domains       []string       `pulumi:"domains"`
    Mode          string         `pulumi:"mode"`
    OrgIds        []string       `pulumi:"orgIds,optional"`
    Configuration map[string]any `pulumi:"configuration"`
}

func (a *CookieConsentConfigArgs) Annotate(ann infer.Annotator) {
    ann.Describe(&a.Name, "The name of the configuration.")
    ann.Describe(&a.Domains, "Domains permitted to host the configuration.")
    ann.Describe(&a.Mode, "Compliance mode: debug, permissive, or production.")
    ann.Describe(&a.OrgIds, "Optional organization IDs associated with the config.")
    ann.Describe(&a.Configuration, "CMP configuration object. At minimum, must include storagePolicyHref.")
}

type CookieConsentConfigState struct {
    CookieConsentConfigArgs

    ConfigId            string `pulumi:"configId"`
    CustomerId          string `pulumi:"customerId,optional"`
    Created             int    `pulumi:"created,optional"`
    Updated             int    `pulumi:"updated,optional"`
    PublishStatus       string `pulumi:"publishStatus,optional"`
    LastPublished       int    `pulumi:"lastPublished,optional"`
    PublishedRevision   int    `pulumi:"publishedRevision,optional"`
    TattleRecordStopped bool   `pulumi:"tattleRecordStopped,optional"`
}

func (s *CookieConsentConfigState) Annotate(ann infer.Annotator) {
    ann.Describe(&s.ConfigId, "The Osano configId (UUID).")
}

// cmpConfigResponse is a minimal shape for the CmpConfigResponse schema.
// We intentionally model only fields we need + allow Configuration passthrough.
type cmpConfigResponse struct {
    Name          string                 `json:"name"`
    Domains       []string               `json:"domains"`
    Mode          string                 `json:"mode"`
    OrgIds        []string               `json:"orgIds"`
    Configuration map[string]any         `json:"configuration"`

    ConfigId            string `json:"configId"`
    CustomerId          string `json:"customerId"`
    Created             int    `json:"created"`
    Updated             int    `json:"updated"`
    PublishStatus       string `json:"publishStatus"`
    LastPublished       int    `json:"lastPublished"`
    PublishedRevision   int    `json:"publishedRevision"`
    TattleRecordStopped bool   `json:"tattleRecordStopped"`
}

func customerClientFromConfig(cfg Config) (*osanoclient.Client, error) {
    if cfg.OsanoApiKey == "" {
        return nil, fmt.Errorf("provider config osanoApiKey is required for Customer REST API operations")
    }
    if cfg.customerBase == nil {
        u, err := url.Parse(cfg.CustomerBaseUrl)
        if err != nil {
            return nil, fmt.Errorf("invalid customerBaseUrl: %w", err)
        }
        cfg.customerBase = u
    }

    return osanoclient.NewClient(cfg.customerBase, "x-osano-api-key", cfg.OsanoApiKey), nil
}

func (r *CookieConsentConfig) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[CookieConsentConfigArgs], error) {
    args, failures, err := infer.DefaultCheck[CookieConsentConfigArgs](ctx, req.NewInputs)
    if err != nil {
        return infer.CheckResponse[CookieConsentConfigArgs]{Inputs: args, Failures: failures}, err
    }

    if args.Name == "" {
        failures = append(failures, p.CheckFailure{Property: "name", Reason: "name is required"})
    }
    if len(args.Domains) == 0 {
        failures = append(failures, p.CheckFailure{Property: "domains", Reason: "at least one domain is required"})
    }
    switch args.Mode {
    case "debug", "permissive", "production":
    default:
        failures = append(failures, p.CheckFailure{Property: "mode", Reason: "mode must be one of: debug, permissive, production"})
    }
    if args.Configuration == nil {
        failures = append(failures, p.CheckFailure{Property: "configuration", Reason: "configuration is required"})
    } else {
        if _, ok := args.Configuration["storagePolicyHref"]; !ok {
            failures = append(failures, p.CheckFailure{Property: "configuration.storagePolicyHref", Reason: "configuration.storagePolicyHref is required"})
        }
    }

    return infer.CheckResponse[CookieConsentConfigArgs]{Inputs: args, Failures: failures}, nil
}

func (r *CookieConsentConfig) Create(ctx context.Context, req infer.CreateRequest[CookieConsentConfigArgs]) (infer.CreateResponse[CookieConsentConfigState], error) {
    if req.DryRun {
        return infer.CreateResponse[CookieConsentConfigState]{ID: "preview"}, nil
    }

    cfg := infer.GetConfig[Config](ctx)
    c, err := customerClientFromConfig(cfg)
    if err != nil {
        return infer.CreateResponse[CookieConsentConfigState]{}, err
    }

    body := map[string]any{
        "name":          req.Inputs.Name,
        "domains":       req.Inputs.Domains,
        "mode":          req.Inputs.Mode,
        "orgIds":        req.Inputs.OrgIds,
        "configuration": req.Inputs.Configuration,
    }

    var out cmpConfigResponse
    if err := c.DoJSON(ctx, "POST", "/v1/cookie-consent/configs", nil, body, &out); err != nil {
        return infer.CreateResponse[CookieConsentConfigState]{}, err
    }

    state := CookieConsentConfigState{
        CookieConsentConfigArgs: CookieConsentConfigArgs{
            Name:          out.Name,
            Domains:       out.Domains,
            Mode:          out.Mode,
            OrgIds:        out.OrgIds,
            Configuration: out.Configuration,
        },
        ConfigId:            out.ConfigId,
        CustomerId:          out.CustomerId,
        Created:             out.Created,
        Updated:             out.Updated,
        PublishStatus:       out.PublishStatus,
        LastPublished:       out.LastPublished,
        PublishedRevision:   out.PublishedRevision,
        TattleRecordStopped: out.TattleRecordStopped,
    }

    return infer.CreateResponse[CookieConsentConfigState]{ID: out.ConfigId, Output: state}, nil
}

func (r *CookieConsentConfig) Read(ctx context.Context, req infer.ReadRequest[CookieConsentConfigArgs, CookieConsentConfigState]) (infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState], error) {
    cfg := infer.GetConfig[Config](ctx)
    c, err := customerClientFromConfig(cfg)
    if err != nil {
        return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{}, err
    }

    var out cmpConfigResponse
    if err := c.DoJSON(ctx, "GET", "/v1/cookie-consent/configs/"+url.PathEscape(req.ID), nil, nil, &out); err != nil {
        return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{}, err
    }

    // Preserve stored inputs (so users can provide partial configuration maps),
    // but refresh the actual remote state.
    state := CookieConsentConfigState{
        CookieConsentConfigArgs: CookieConsentConfigArgs{
            Name:          out.Name,
            Domains:       out.Domains,
            Mode:          out.Mode,
            OrgIds:        out.OrgIds,
            Configuration: out.Configuration,
        },
        ConfigId:            out.ConfigId,
        CustomerId:          out.CustomerId,
        Created:             out.Created,
        Updated:             out.Updated,
        PublishStatus:       out.PublishStatus,
        LastPublished:       out.LastPublished,
        PublishedRevision:   out.PublishedRevision,
        TattleRecordStopped: out.TattleRecordStopped,
    }

    return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{
        ID:     out.ConfigId,
        Inputs: req.Inputs,
        State:  state,
    }, nil
}

func (r *CookieConsentConfig) Update(ctx context.Context, req infer.UpdateRequest[CookieConsentConfigArgs, CookieConsentConfigState]) (infer.UpdateResponse[CookieConsentConfigState], error) {
    if req.DryRun {
        return infer.UpdateResponse[CookieConsentConfigState]{Output: req.State}, nil
    }

    cfg := infer.GetConfig[Config](ctx)
    c, err := customerClientFromConfig(cfg)
    if err != nil {
        return infer.UpdateResponse[CookieConsentConfigState]{}, err
    }

    body := map[string]any{
        "name":          req.Inputs.Name,
        "domains":       req.Inputs.Domains,
        "mode":          req.Inputs.Mode,
        "orgIds":        req.Inputs.OrgIds,
        "configuration": req.Inputs.Configuration,
    }

    var out cmpConfigResponse
    if err := c.DoJSON(ctx, "PATCH", "/v1/cookie-consent/configs/"+url.PathEscape(req.ID), nil, body, &out); err != nil {
        return infer.UpdateResponse[CookieConsentConfigState]{}, err
    }

    state := CookieConsentConfigState{
        CookieConsentConfigArgs: CookieConsentConfigArgs{
            Name:          out.Name,
            Domains:       out.Domains,
            Mode:          out.Mode,
            OrgIds:        out.OrgIds,
            Configuration: out.Configuration,
        },
        ConfigId:            out.ConfigId,
        CustomerId:          out.CustomerId,
        Created:             out.Created,
        Updated:             out.Updated,
        PublishStatus:       out.PublishStatus,
        LastPublished:       out.LastPublished,
        PublishedRevision:   out.PublishedRevision,
        TattleRecordStopped: out.TattleRecordStopped,
    }

    return infer.UpdateResponse[CookieConsentConfigState]{Output: state}, nil
}

func (r *CookieConsentConfig) Delete(ctx context.Context, req infer.DeleteRequest[CookieConsentConfigState]) (infer.DeleteResponse, error) {
    // No delete endpoint in the public spec. Intentionally a no-op.
    p.GetLogger(ctx).Warningf("Osano API does not provide a delete endpoint for cookie consent configs; leaving remote config %q", req.ID)
    return infer.DeleteResponse{}, nil
}

func (r *CookieConsentConfig) Diff(ctx context.Context, req infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState]) (infer.DiffResponse, error) {
    diff := map[string]p.PropertyDiff{}

    if req.Inputs.Name != req.State.Name {
        diff["name"] = p.PropertyDiff{Kind: p.Update}
    }
    if !slices.Equal(req.Inputs.Domains, req.State.Domains) {
        diff["domains"] = p.PropertyDiff{Kind: p.Update}
    }
    if req.Inputs.Mode != req.State.Mode {
        diff["mode"] = p.PropertyDiff{Kind: p.Update}
    }
    if !slices.Equal(req.Inputs.OrgIds, req.State.OrgIds) {
        diff["orgIds"] = p.PropertyDiff{Kind: p.Update}
    }

    // configuration can be huge; only diff keys the user provided.
    if req.Inputs.Configuration != nil {
        for k, desired := range req.Inputs.Configuration {
            current, ok := req.State.Configuration[k]
            if !ok {
                diff["configuration"] = p.PropertyDiff{Kind: p.Update}
                break
            }
            db, errD := json.Marshal(desired)
            cb, errC := json.Marshal(current)
            if errD != nil || errC != nil || !bytes.Equal(db, cb) {
                diff["configuration"] = p.PropertyDiff{Kind: p.Update}
                break
            }
        }
    }

    return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}
