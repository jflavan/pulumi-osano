package provider

import (
	"context"
	"fmt"
	"strconv"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// CookieConsentRule manages a Cookie Consent (CMP) rule via the Osano Customer REST API.
//
// Create: POST /v1/cookie-consent/rules  (body includes configIds + scripts/cookies/iframes/localStorage)
// Read:   GET  /v1/cookie-consent/configs/{configId}/rules  (list and find by ruleId)
// Update: PATCH /v1/cookie-consent/rules/{ruleId}
// Delete: DELETE /v1/cookie-consent/rules/{ruleId}
type CookieConsentRule struct{}

var _ = (infer.CustomCheck[CookieConsentRuleArgs])((*CookieConsentRule)(nil))
var _ = (infer.CustomDiff[CookieConsentRuleArgs, CookieConsentRuleState])((*CookieConsentRule)(nil))
var _ = (infer.CustomRead[CookieConsentRuleArgs, CookieConsentRuleState])((*CookieConsentRule)(nil))
var _ = (infer.CustomUpdate[CookieConsentRuleArgs, CookieConsentRuleState])((*CookieConsentRule)(nil))
var _ = (infer.CustomDelete[CookieConsentRuleState])((*CookieConsentRule)(nil))
var _ = (infer.Annotated)((*CookieConsentRule)(nil))
var _ = (infer.Annotated)((*CookieConsentRuleArgs)(nil))
var _ = (infer.Annotated)((*CookieConsentRuleState)(nil))

func (r *CookieConsentRule) Annotate(a infer.Annotator) {
	a.Describe(&r, "Manages an Osano Cookie Consent (CMP) rule within a configuration.")
}

// CookieConsentRuleArgs are the user-provided inputs.
type CookieConsentRuleArgs struct {
	ConfigId       string  `pulumi:"configId"`
	StoreType      string  `pulumi:"storeType"`
	Classification string  `pulumi:"classification"`
	Rule           string  `pulumi:"rule"`
	Disclosure     bool    `pulumi:"disclosure,optional"`
	Title          *string `pulumi:"title,optional"`
	VendorName     *string `pulumi:"vendorName,optional"`
}

func (a *CookieConsentRuleArgs) Annotate(ann infer.Annotator) {
	ann.Describe(&a.ConfigId, "The configId of the Cookie Consent Configuration this rule belongs to.")
	ann.Describe(&a.StoreType, "The storage type category: cookies, scripts, iframes, or localStorage.")
	ann.Describe(&a.Classification, "Classification: ANALYTICS, BLACKLISTED, ESSENTIAL, HIDDEN, MARKETING, or PERSONALIZATION.")
	ann.Describe(&a.Rule, "The rule pattern (e.g. a cookie name pattern). Min 3, max 1000 characters.")
	ann.Describe(&a.Disclosure, "Whether the rule should be disclosed. Defaults to false.")
	ann.Describe(&a.Title, "Optional title for the rule, used in consent disclosure.")
	ann.Describe(&a.VendorName, "Optional vendor name for the rule.")
}

// CookieConsentRuleState extends args with server-returned fields.
type CookieConsentRuleState struct {
	CookieConsentRuleArgs

	RuleId  int    `pulumi:"ruleId"`
	Created string `pulumi:"created,optional"`
	Updated string `pulumi:"updated,optional"`
}

func (s *CookieConsentRuleState) Annotate(ann infer.Annotator) {
	ann.Describe(&s.RuleId, "The server-assigned integer rule ID.")
	ann.Describe(&s.Created, "Timestamp when the rule was created.")
	ann.Describe(&s.Updated, "Timestamp when the rule was last updated.")
}

// cmpRuleResponse matches the response shape: CmpRule + CmpRuleResponseProperties merged.
type cmpRuleResponse struct {
	Classification string  `json:"classification"`
	Rule           string  `json:"rule"`
	Disclosure     bool    `json:"disclosure"`
	Title          *string `json:"title"`
	VendorName     *string `json:"vendorName"`

	Type     string `json:"type"`
	RuleId   int    `json:"ruleId"`
	ConfigId string `json:"configId"`
	VendorId string `json:"vendorId"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

// cmpRulesListResponse wraps the items array returned by list and create endpoints.
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

func (r *CookieConsentRule) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[CookieConsentRuleArgs], error) {
	args, failures, err := infer.DefaultCheck[CookieConsentRuleArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[CookieConsentRuleArgs]{Inputs: args, Failures: failures}, err
	}

	if args.ConfigId == "" {
		failures = append(failures, p.CheckFailure{Property: "configId", Reason: "configId is required"})
	}
	if args.StoreType == "" {
		failures = append(failures, p.CheckFailure{Property: "storeType", Reason: "storeType is required"})
	} else if !validStoreTypes[args.StoreType] {
		failures = append(failures, p.CheckFailure{Property: "storeType", Reason: "storeType must be one of: cookies, scripts, iframes, localStorage"})
	}
	if args.Classification == "" {
		failures = append(failures, p.CheckFailure{Property: "classification", Reason: "classification is required"})
	} else if !validClassifications[args.Classification] {
		failures = append(failures, p.CheckFailure{Property: "classification", Reason: "classification must be one of: ANALYTICS, BLACKLISTED, ESSENTIAL, HIDDEN, MARKETING, PERSONALIZATION"})
	}
	if args.Rule == "" {
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule is required"})
	} else if len(args.Rule) < 3 {
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at least 3 characters"})
	} else if len(args.Rule) > 1000 {
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at most 1000 characters"})
	}

	return infer.CheckResponse[CookieConsentRuleArgs]{Inputs: args, Failures: failures}, nil
}

func (r *CookieConsentRule) Create(ctx context.Context, req infer.CreateRequest[CookieConsentRuleArgs]) (infer.CreateResponse[CookieConsentRuleState], error) {
	if req.DryRun {
		return infer.CreateResponse[CookieConsentRuleState]{ID: "preview"}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	c, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, err
	}

	// Build the rule object for the create request.
	ruleObj := map[string]any{
		"classification": req.Inputs.Classification,
		"rule":           req.Inputs.Rule,
		"disclosure":     req.Inputs.Disclosure,
	}
	if req.Inputs.Title != nil {
		ruleObj["title"] = *req.Inputs.Title
	}
	if req.Inputs.VendorName != nil {
		ruleObj["vendorName"] = *req.Inputs.VendorName
	}

	// POST /v1/cookie-consent/rules body: { configIds: [...], <storeType>: [{rule}] }
	body := map[string]any{
		"configIds":          []string{req.Inputs.ConfigId},
		req.Inputs.StoreType: []any{ruleObj},
	}

	var out cmpRulesListResponse
	if err := c.DoJSON(ctx, "POST", "/v1/cookie-consent/rules", nil, body, &out); err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, fmt.Errorf("create rule: %w", err)
	}

	if len(out.Items) == 0 {
		return infer.CreateResponse[CookieConsentRuleState]{}, fmt.Errorf("create rule: API returned empty items list")
	}

	created := out.Items[0]
	id := strconv.Itoa(created.RuleId)

	state := ruleResponseToState(req.Inputs, created)

	return infer.CreateResponse[CookieConsentRuleState]{ID: id, Output: state}, nil
}

func (r *CookieConsentRule) Read(ctx context.Context, req infer.ReadRequest[CookieConsentRuleArgs, CookieConsentRuleState]) (infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState], error) {
	cfg := infer.GetConfig[Config](ctx)
	c, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, err
	}

	// Parse the ruleId from the resource ID.
	ruleId, err := strconv.Atoi(req.ID)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("invalid rule ID %q: %w", req.ID, err)
	}

	configId := req.State.ConfigId
	if configId == "" {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("configId is required for reading a rule")
	}

	// List rules for the config and find ours.
	var out cmpRulesListResponse
	if err := c.DoJSON(ctx, "GET", "/v1/cookie-consent/configs/"+configId+"/rules", nil, nil, &out); err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("read rule: %w", err)
	}

	for _, item := range out.Items {
		if item.RuleId == ruleId {
			state := ruleResponseToState(req.Inputs, item)
			return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{
				ID:     req.ID,
				Inputs: req.Inputs,
				State:  state,
			}, nil
		}
	}

	// Rule not found — deleted out of band.
	return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{ID: ""}, nil
}

func (r *CookieConsentRule) Update(ctx context.Context, req infer.UpdateRequest[CookieConsentRuleArgs, CookieConsentRuleState]) (infer.UpdateResponse[CookieConsentRuleState], error) {
	if req.DryRun {
		return infer.UpdateResponse[CookieConsentRuleState]{Output: req.State}, nil
	}

	cfg := infer.GetConfig[Config](ctx)
	c, err := customerClientFromConfig(cfg)
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
	if err := c.DoJSON(ctx, "PATCH", "/v1/cookie-consent/rules/"+req.ID, nil, body, &out); err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, fmt.Errorf("update rule: %w", err)
	}

	state := ruleResponseToState(req.Inputs, out)

	return infer.UpdateResponse[CookieConsentRuleState]{Output: state}, nil
}

func (r *CookieConsentRule) Delete(ctx context.Context, req infer.DeleteRequest[CookieConsentRuleState]) (infer.DeleteResponse, error) {
	cfg := infer.GetConfig[Config](ctx)
	c, err := customerClientFromConfig(cfg)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	id := strconv.Itoa(req.State.RuleId)
	if err := c.DoJSON(ctx, "DELETE", "/v1/cookie-consent/rules/"+id, nil, nil, nil); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("delete rule: %w", err)
	}

	return infer.DeleteResponse{}, nil
}

func (r *CookieConsentRule) Diff(ctx context.Context, req infer.DiffRequest[CookieConsentRuleArgs, CookieConsentRuleState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}

	// configId and storeType changes require replacement.
	if req.Inputs.ConfigId != req.State.ConfigId {
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
		HasChanges:  len(diff) > 0,
		DetailedDiff: diff,
	}, nil
}

// ruleResponseToState converts an API response + user inputs into state.
func ruleResponseToState(inputs CookieConsentRuleArgs, resp cmpRuleResponse) CookieConsentRuleState {
	return CookieConsentRuleState{
		CookieConsentRuleArgs: CookieConsentRuleArgs{
			ConfigId:       inputs.ConfigId,
			StoreType:      inputs.StoreType,
			Classification: resp.Classification,
			Rule:           resp.Rule,
			Disclosure:     resp.Disclosure,
			Title:          resp.Title,
			VendorName:     resp.VendorName,
		},
		RuleId:  resp.RuleId,
		Created: resp.Created,
		Updated: resp.Updated,
	}
}

// ptrStringEqual compares two *string values (nil-safe).
func ptrStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
