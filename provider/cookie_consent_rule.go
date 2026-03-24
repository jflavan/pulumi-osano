//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// CookieConsentRule manages a Cookie Consent rule within an Osano CMP configuration.
type CookieConsentRule struct{}

// CookieConsentRuleArgs are the user inputs for a CMP rule.
type CookieConsentRuleArgs struct {
	ConfigID       string  `pulumi:"configId"`
	StoreType      string  `pulumi:"storeType"`
	Classification string  `pulumi:"classification"`
	Rule           string  `pulumi:"rule"`
	Disclosure     bool    `pulumi:"disclosure,optional"`
	Title          *string `pulumi:"title,optional"`
	VendorName     *string `pulumi:"vendorName,optional"`
}

// CookieConsentRuleState extends the args with server-managed metadata.
type CookieConsentRuleState struct {
	CookieConsentRuleArgs

	RuleID  int    `pulumi:"ruleId"`
	Created string `pulumi:"created,optional"`
	Updated string `pulumi:"updated,optional"`
}

// Annotate documents the CookieConsentRule resource.
func (r *CookieConsentRule) Annotate(a infer.Annotator) {
	a.Describe(r, "Manages an Osano Cookie Consent (CMP) rule within a configuration.")
}

// Annotate documents the CookieConsentRule input fields.
func (args *CookieConsentRuleArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ConfigID, "The configId of the Cookie Consent Configuration this rule belongs to.")
	a.Describe(&args.StoreType, "The storage type category: cookies, scripts, iframes, or localStorage.")
	a.Describe(
		&args.Classification,
		"Classification: ANALYTICS, BLACKLISTED, ESSENTIAL, HIDDEN, MARKETING, or PERSONALIZATION.",
	)
	a.Describe(&args.Rule, "The rule pattern (e.g. a cookie name pattern). Min 3, max 1000 characters.")
	a.Describe(&args.Disclosure, "Whether the rule should be disclosed. Defaults to false.")
	a.Describe(&args.Title, "Optional title for the rule, used in consent disclosure.")
	a.Describe(&args.VendorName, "Optional vendor name for the rule.")
}

// Annotate documents the CookieConsentRule state fields.
func (state *CookieConsentRuleState) Annotate(a infer.Annotator) {
	a.Describe(&state.RuleID, "The server-assigned integer rule ID.")
	a.Describe(&state.Created, "Timestamp when the rule was created.")
	a.Describe(&state.Updated, "Timestamp when the rule was last updated.")
}

type cmpRuleResponse struct {
	Classification string  `json:"classification"`
	Rule           string  `json:"rule"`
	Disclosure     bool    `json:"disclosure"`
	Title          *string `json:"title"`
	VendorName     *string `json:"vendorName"`

	Type     string `json:"type"`
	RuleID   int    `json:"ruleId"`
	ConfigID string `json:"configId"`
	VendorID string `json:"vendorId"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

type cmpRulesListResponse struct {
	Items []cmpRuleResponse `json:"items"`
}

var validClassifications = map[string]bool{
	"ANALYTICS":       true,
	"BLACKLISTED":     true,
	"ESSENTIAL":       true,
	"HIDDEN":          true,
	"MARKETING":       true,
	"PERSONALIZATION": true,
}

var validStoreTypes = map[string]bool{
	"cookies":      true,
	"scripts":      true,
	"iframes":      true,
	"localStorage": true,
}

// Check validates CookieConsentRule inputs before create or update.
func (r *CookieConsentRule) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CookieConsentRuleArgs], error) {
	args, failures, err := infer.DefaultCheck[CookieConsentRuleArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[CookieConsentRuleArgs]{Inputs: args, Failures: failures}, err
	}

	if args.ConfigID == "" {
		failures = append(failures, p.CheckFailure{Property: "configId", Reason: "configId is required"})
	}
	if args.StoreType == "" {
		failures = append(failures, p.CheckFailure{Property: "storeType", Reason: "storeType is required"})
	} else if !validStoreTypes[args.StoreType] {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "storeType",
				Reason:   "storeType must be one of: cookies, scripts, iframes, localStorage",
			},
		)
	}
	if args.Classification == "" {
		failures = append(failures, p.CheckFailure{Property: "classification", Reason: "classification is required"})
	} else if !validClassifications[args.Classification] {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "classification",
				Reason:   "classification must be one of: ANALYTICS, BLACKLISTED, ESSENTIAL, HIDDEN, MARKETING, PERSONALIZATION",
			},
		)
	}
	switch {
	case args.Rule == "":
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule is required"})
	case len(args.Rule) < 3:
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at least 3 characters"})
	case len(args.Rule) > 1000:
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at most 1000 characters"})
	}

	return infer.CheckResponse[CookieConsentRuleArgs]{Inputs: args, Failures: failures}, nil
}

// Create provisions a CookieConsentRule via the Customer REST API.
func (r *CookieConsentRule) Create(
	ctx context.Context, req infer.CreateRequest[CookieConsentRuleArgs],
) (infer.CreateResponse[CookieConsentRuleState], error) {
	if req.DryRun {
		return infer.CreateResponse[CookieConsentRuleState]{ID: "preview"}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, err
	}

	ruleObject := map[string]any{
		"classification": req.Inputs.Classification,
		"rule":           req.Inputs.Rule,
		"disclosure":     req.Inputs.Disclosure,
	}
	if req.Inputs.Title != nil {
		ruleObject["title"] = *req.Inputs.Title
	}
	if req.Inputs.VendorName != nil {
		ruleObject["vendorName"] = *req.Inputs.VendorName
	}

	body := map[string]any{
		"configIds":          []string{req.Inputs.ConfigID},
		req.Inputs.StoreType: []any{ruleObject},
	}

	var out cmpRulesListResponse
	if err := client.DoJSON(ctx, "POST", "/v1/cookie-consent/rules", nil, body, &out); err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, fmt.Errorf("create rule: %w", err)
	}
	if len(out.Items) == 0 {
		return infer.CreateResponse[CookieConsentRuleState]{}, errors.New("create rule: API returned empty items list")
	}

	created := out.Items[0]
	return infer.CreateResponse[CookieConsentRuleState]{
		ID:     strconv.Itoa(created.RuleID),
		Output: ruleResponseToState(req.Inputs, created),
	}, nil
}

// Read refreshes the tracked CookieConsentRule from the Customer REST API.
func (r *CookieConsentRule) Read(
	ctx context.Context, req infer.ReadRequest[CookieConsentRuleArgs, CookieConsentRuleState],
) (infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState], error) {
	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, err
	}

	ruleID, err := strconv.Atoi(req.ID)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{},
			fmt.Errorf("invalid rule ID %q: %w", req.ID, err)
	}

	configID := req.State.ConfigID
	if configID == "" {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{},
			errors.New("configId is required for reading a rule")
	}

	var out cmpRulesListResponse
	if err := client.DoJSON(
		ctx,
		"GET",
		"/v1/cookie-consent/configs/"+url.PathEscape(configID)+"/rules",
		nil,
		nil,
		&out,
	); err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("read rule: %w", err)
	}

	for idx := range out.Items {
		item := out.Items[idx]
		if item.RuleID == ruleID {
			return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{
				ID:     req.ID,
				Inputs: req.Inputs,
				State:  ruleResponseToState(req.Inputs, item),
			}, nil
		}
	}

	return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{ID: ""}, nil
}

// Update applies mutable CookieConsentRule changes in place.
func (r *CookieConsentRule) Update(
	ctx context.Context, req infer.UpdateRequest[CookieConsentRuleArgs, CookieConsentRuleState],
) (infer.UpdateResponse[CookieConsentRuleState], error) {
	if req.DryRun {
		return infer.UpdateResponse[CookieConsentRuleState]{Output: req.State}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, err
	}

	body := map[string]any{
		"classification": req.Inputs.Classification,
		"rule":           req.Inputs.Rule,
		"disclosure":     req.Inputs.Disclosure,
	}
	if req.Inputs.Title != nil {
		body["title"] = *req.Inputs.Title
	}
	if req.Inputs.VendorName != nil {
		body["vendorName"] = *req.Inputs.VendorName
	}

	var out cmpRuleResponse
	if err := client.DoJSON(
		ctx,
		"PATCH",
		"/v1/cookie-consent/rules/"+url.PathEscape(req.ID),
		nil,
		body,
		&out,
	); err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, fmt.Errorf("update rule: %w", err)
	}

	return infer.UpdateResponse[CookieConsentRuleState]{Output: ruleResponseToState(req.Inputs, out)}, nil
}

// Delete removes the tracked CookieConsentRule from the Customer REST API.
func (r *CookieConsentRule) Delete(
	ctx context.Context, req infer.DeleteRequest[CookieConsentRuleState],
) (infer.DeleteResponse, error) {
	cfg := infer.GetConfig[Config](ctx)
	client, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	if err := client.DoJSON(
		ctx,
		"DELETE",
		"/v1/cookie-consent/rules/"+url.PathEscape(strconv.Itoa(req.State.RuleID)),
		nil,
		nil,
		nil,
	); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("delete rule: %w", err)
	}

	return infer.DeleteResponse{}, nil
}

// Diff reports in-place updates versus replacements for CookieConsentRule fields.
func (r *CookieConsentRule) Diff(
	_ context.Context, req infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState],
) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}

	if req.Inputs.ConfigID != req.State.ConfigID {
		diff["configId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.StoreType != req.State.StoreType {
		diff["storeType"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.Classification != req.State.Classification {
		diff["classification"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.Inputs.Rule != req.State.Rule {
		diff["rule"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.Inputs.Disclosure != req.State.Disclosure {
		diff["disclosure"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.Title, req.State.Title) {
		diff["title"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.VendorName, req.State.VendorName) {
		diff["vendorName"] = p.PropertyDiff{Kind: p.Update}
	}

	return infer.DiffResponse{
		HasChanges:   len(diff) > 0,
		DetailedDiff: diff,
	}, nil
}

func ruleResponseToState(inputs CookieConsentRuleArgs, resp cmpRuleResponse) CookieConsentRuleState {
	return CookieConsentRuleState{
		CookieConsentRuleArgs: CookieConsentRuleArgs{
			ConfigID:       inputs.ConfigID,
			StoreType:      inputs.StoreType,
			Classification: resp.Classification,
			Rule:           resp.Rule,
			Disclosure:     resp.Disclosure,
			Title:          resp.Title,
			VendorName:     resp.VendorName,
		},
		RuleID:  resp.RuleID,
		Created: resp.Created,
		Updated: resp.Updated,
	}
}

func ptrStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
