//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"

	osanoclient "github.com/jflavan/pulumi-osano/provider/internal/osano"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// CookieConsentConfig manages a Cookie Consent configuration via the Osano Customer REST API.
type CookieConsentConfig struct{}

// CookieConsentConfigArgs captures the desired Cookie Consent configuration.
type CookieConsentConfigArgs struct {
	Name          string         `pulumi:"name"`
	Domains       []string       `pulumi:"domains"`
	Mode          string         `pulumi:"mode"`
	OrgIDs        []string       `pulumi:"orgIds,optional"`
	Configuration map[string]any `pulumi:"configuration"`
}

// CookieConsentConfigState extends the user inputs with Osano-managed metadata.
type CookieConsentConfigState struct {
	CookieConsentConfigArgs

	ConfigID            string `pulumi:"configId"`
	CustomerID          string `pulumi:"customerId,optional"`
	Created             int    `pulumi:"created,optional"`
	Updated             int    `pulumi:"updated,optional"`
	PublishStatus       string `pulumi:"publishStatus,optional"`
	LastPublished       int    `pulumi:"lastPublished,optional"`
	PublishedRevision   int    `pulumi:"publishedRevision,optional"`
	TattleRecordStopped bool   `pulumi:"tattleRecordStopped,optional"`
}

// Annotate documents the CookieConsentConfig resource.
func (r *CookieConsentConfig) Annotate(a infer.Annotator) {
	a.Describe(r, "Manages an Osano Cookie Consent (CMP) configuration.")
}

// Annotate documents the CookieConsentConfig input fields.
func (args *CookieConsentConfigArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the configuration.")
	a.Describe(&args.Domains, "Domains permitted to host the configuration.")
	a.Describe(&args.Mode, "Compliance mode: debug, permissive, or production.")
	a.Describe(&args.OrgIDs, "Optional organization IDs associated with the config.")
	a.Describe(
		&args.Configuration,
		"CMP configuration object. At minimum, must include storagePolicyHref.",
	)
}

// Annotate documents the CookieConsentConfig state fields.
func (state *CookieConsentConfigState) Annotate(a infer.Annotator) {
	a.Describe(&state.ConfigID, "The Osano configId (UUID).")
}

type cmpConfigResponse struct {
	Name          string         `json:"name"`
	Domains       []string       `json:"domains"`
	Mode          string         `json:"mode"`
	OrgIDs        []string       `json:"orgIds"`
	Configuration map[string]any `json:"configuration"`

	ConfigID            string `json:"configId"`
	CustomerID          string `json:"customerId"`
	Created             int    `json:"created"`
	Updated             int    `json:"updated"`
	PublishStatus       string `json:"publishStatus"`
	LastPublished       int    `json:"lastPublished"`
	PublishedRevision   int    `json:"publishedRevision"`
	TattleRecordStopped bool   `json:"tattleRecordStopped"`
}

func customerClientFromConfig(cfg Config) (*osanoclient.Client, error) {
	if cfg.OsanoAPIKey == "" {
		return nil, errors.New("provider config osanoApiKey is required for Customer REST API operations")
	}

	baseURL, err := url.Parse(cfg.customerBaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid customerBaseUrl: %w", err)
	}

	return osanoclient.NewClient(baseURL, "x-osano-api-key", cfg.OsanoAPIKey), nil
}

// Check validates CookieConsentConfig inputs before create or update.
func (r *CookieConsentConfig) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CookieConsentConfigArgs], error) {
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
		failures = append(
			failures,
			p.CheckFailure{
				Property: "mode",
				Reason:   "mode must be one of: debug, permissive, production",
			},
		)
	}
	if args.Configuration == nil {
		failures = append(failures, p.CheckFailure{Property: "configuration", Reason: "configuration is required"})
	} else if _, ok := args.Configuration["storagePolicyHref"]; !ok {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "configuration.storagePolicyHref",
				Reason:   "configuration.storagePolicyHref is required",
			},
		)
	}

	return infer.CheckResponse[CookieConsentConfigArgs]{Inputs: args, Failures: failures}, nil
}

// Create provisions a CookieConsentConfig via the Customer REST API.
func (r *CookieConsentConfig) Create(
	ctx context.Context, req infer.CreateRequest[CookieConsentConfigArgs],
) (infer.CreateResponse[CookieConsentConfigState], error) {
	if req.DryRun {
		return infer.CreateResponse[CookieConsentConfigState]{ID: "preview"}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.CreateResponse[CookieConsentConfigState]{}, err
	}

	body := map[string]any{
		"name":          req.Inputs.Name,
		"domains":       req.Inputs.Domains,
		"mode":          req.Inputs.Mode,
		"orgIds":        req.Inputs.OrgIDs,
		"configuration": req.Inputs.Configuration,
	}

	var out cmpConfigResponse
	if err := client.DoJSON(ctx, "POST", "/v1/cookie-consent/configs", nil, body, &out); err != nil {
		return infer.CreateResponse[CookieConsentConfigState]{}, err
	}

	state := cookieConsentConfigStateFromResponse(out)
	return infer.CreateResponse[CookieConsentConfigState]{ID: out.ConfigID, Output: state}, nil
}

// Read refreshes the tracked CookieConsentConfig from the Customer REST API.
func (r *CookieConsentConfig) Read(
	ctx context.Context, req infer.ReadRequest[CookieConsentConfigArgs, CookieConsentConfigState],
) (infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState], error) {
	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{}, err
	}

	var out cmpConfigResponse
	if err := client.DoJSON(
		ctx,
		"GET",
		"/v1/cookie-consent/configs/"+url.PathEscape(req.ID),
		nil,
		nil,
		&out,
	); err != nil {
		return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{}, err
	}

	return infer.ReadResponse[CookieConsentConfigArgs, CookieConsentConfigState]{
		ID:     out.ConfigID,
		Inputs: req.Inputs,
		State:  cookieConsentConfigStateFromResponse(out),
	}, nil
}

// Update applies mutable CookieConsentConfig changes in place.
func (r *CookieConsentConfig) Update(
	ctx context.Context, req infer.UpdateRequest[CookieConsentConfigArgs, CookieConsentConfigState],
) (infer.UpdateResponse[CookieConsentConfigState], error) {
	if req.DryRun {
		return infer.UpdateResponse[CookieConsentConfigState]{Output: req.State}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.UpdateResponse[CookieConsentConfigState]{}, err
	}

	body := map[string]any{
		"name":          req.Inputs.Name,
		"domains":       req.Inputs.Domains,
		"mode":          req.Inputs.Mode,
		"orgIds":        req.Inputs.OrgIDs,
		"configuration": req.Inputs.Configuration,
	}

	var out cmpConfigResponse
	if err := client.DoJSON(
		ctx,
		"PATCH",
		"/v1/cookie-consent/configs/"+url.PathEscape(req.ID),
		nil,
		body,
		&out,
	); err != nil {
		return infer.UpdateResponse[CookieConsentConfigState]{}, err
	}

	return infer.UpdateResponse[CookieConsentConfigState]{
		Output: cookieConsentConfigStateFromResponse(out),
	}, nil
}

// Delete forgets the local CookieConsentConfig state without removing the upstream config.
func (r *CookieConsentConfig) Delete(
	_ context.Context, _ infer.DeleteRequest[CookieConsentConfigState],
) (infer.DeleteResponse, error) {
	// Osano does not expose a delete endpoint for CMP configs, so destroy only forgets local state.
	return infer.DeleteResponse{}, nil
}

// Diff reports in-place changes for CookieConsentConfig fields.
func (r *CookieConsentConfig) Diff(
	_ context.Context, req infer.DiffRequest[CookieConsentConfigArgs, CookieConsentConfigState],
) (infer.DiffResponse, error) {
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
	if !slices.Equal(req.Inputs.OrgIDs, req.State.OrgIDs) {
		diff["orgIds"] = p.PropertyDiff{Kind: p.Update}
	}

	if req.Inputs.Configuration != nil {
		for key, desired := range req.Inputs.Configuration {
			current, ok := req.State.Configuration[key]
			if !ok {
				diff["configuration"] = p.PropertyDiff{Kind: p.Update}
				break
			}
			desiredBytes, errDesired := json.Marshal(desired)
			currentBytes, errCurrent := json.Marshal(current)
			if errDesired != nil || errCurrent != nil || !bytes.Equal(desiredBytes, currentBytes) {
				diff["configuration"] = p.PropertyDiff{Kind: p.Update}
				break
			}
		}
	}

	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func cookieConsentConfigStateFromResponse(resp cmpConfigResponse) CookieConsentConfigState {
	return CookieConsentConfigState{
		CookieConsentConfigArgs: CookieConsentConfigArgs{
			Name:          resp.Name,
			Domains:       resp.Domains,
			Mode:          resp.Mode,
			OrgIDs:        resp.OrgIDs,
			Configuration: resp.Configuration,
		},
		ConfigID:            resp.ConfigID,
		CustomerID:          resp.CustomerID,
		Created:             resp.Created,
		Updated:             resp.Updated,
		PublishStatus:       resp.PublishStatus,
		LastPublished:       resp.LastPublished,
		PublishedRevision:   resp.PublishedRevision,
		TattleRecordStopped: resp.TattleRecordStopped,
	}
}
