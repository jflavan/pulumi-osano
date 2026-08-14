//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	osanoclient "github.com/jflavan/pulumi-osano/provider/internal/osano"

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
	RuleType       *string `pulumi:"ruleType,optional"`
	Description    *string `pulumi:"description,optional"`
	Expiry         *string `pulumi:"expiry,optional"`
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
	a.Describe(&args.Title, "Optional title for the rule, used in consent disclosure. Max 64 characters.")
	a.Describe(&args.VendorName, "Optional vendor name for the rule. Max 100 characters.")
	a.Describe(
		&args.RuleType,
		"Optional matching mode: FILENAME, DOMAIN, PATH, REGEXP, STARTS_WITH, ENDS_WITH, CONTAINS, or EXACT_MATCH.",
	)
	a.Describe(&args.Description, "Optional cookie description. Only supported for cookies; max 1000 characters.")
	a.Describe(&args.Expiry, "Optional cookie expiry description. Only supported for cookies; max 50 characters.")
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
	RuleType       *string `json:"ruleType"`
	Description    *string `json:"description"`
	Expiry         *string `json:"expiry"`

	Type     string `json:"type"`
	RuleID   int    `json:"ruleId"`
	ConfigID string `json:"configId"`
	VendorID string `json:"vendorId"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

type cmpRulesListResponse struct {
	Items []cmpRuleResponse `json:"items"`
	Next  string            `json:"next"`
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

var validRuleTypes = map[string]bool{
	"FILENAME":    true,
	"DOMAIN":      true,
	"PATH":        true,
	"REGEXP":      true,
	"STARTS_WITH": true,
	"ENDS_WITH":   true,
	"CONTAINS":    true,
	"EXACT_MATCH": true,
}

// Check validates CookieConsentRule inputs before create or update.
func (r *CookieConsentRule) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CookieConsentRuleArgs], error) {
	args, failures, err := infer.DefaultCheck[CookieConsentRuleArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[CookieConsentRuleArgs]{Inputs: args, Failures: failures}, err
	}

	propertyKnown := func(name string) bool {
		return !req.NewInputs.Get(name).HasComputed()
	}
	storeTypeKnown := propertyKnown("storeType")

	if propertyKnown("configId") && args.ConfigID == "" {
		failures = append(failures, p.CheckFailure{Property: "configId", Reason: "configId is required"})
	}
	if storeTypeKnown && args.StoreType == "" {
		failures = append(failures, p.CheckFailure{Property: "storeType", Reason: "storeType is required"})
	} else if storeTypeKnown && !validStoreTypes[args.StoreType] {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "storeType",
				Reason:   "storeType must be one of: cookies, scripts, iframes, localStorage",
			},
		)
	}
	if propertyKnown("classification") && args.Classification == "" {
		failures = append(failures, p.CheckFailure{Property: "classification", Reason: "classification is required"})
	} else if propertyKnown("classification") && !validClassifications[args.Classification] {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "classification",
				Reason:   "classification must be one of: ANALYTICS, BLACKLISTED, ESSENTIAL, HIDDEN, MARKETING, PERSONALIZATION",
			},
		)
	}
	switch {
	case !propertyKnown("rule"):
	case args.Rule == "":
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule is required"})
	case len(args.Rule) < 3:
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at least 3 characters"})
	case len(args.Rule) > 1000:
		failures = append(failures, p.CheckFailure{Property: "rule", Reason: "rule must be at most 1000 characters"})
	}
	if propertyKnown("title") && args.Title != nil && len(*args.Title) > 64 {
		failures = append(failures, p.CheckFailure{Property: "title", Reason: "title must be at most 64 characters"})
	}
	if propertyKnown("vendorName") && args.VendorName != nil && len(*args.VendorName) > 100 {
		failures = append(
			failures,
			p.CheckFailure{Property: "vendorName", Reason: "vendorName must be at most 100 characters"},
		)
	}
	if propertyKnown("ruleType") && args.RuleType != nil && !validRuleTypes[*args.RuleType] {
		failures = append(
			failures,
			p.CheckFailure{
				Property: "ruleType",
				Reason:   "ruleType must be one of: FILENAME, DOMAIN, PATH, REGEXP, STARTS_WITH, ENDS_WITH, CONTAINS, EXACT_MATCH",
			},
		)
	}
	if propertyKnown("description") && args.Description != nil {
		if len(*args.Description) > 1000 {
			failures = append(
				failures,
				p.CheckFailure{Property: "description", Reason: "description must be at most 1000 characters"},
			)
		}
		if storeTypeKnown && args.StoreType != "cookies" {
			failures = append(
				failures,
				p.CheckFailure{Property: "description", Reason: "description is only supported for cookies"},
			)
		}
	}
	if propertyKnown("expiry") && args.Expiry != nil {
		if len(*args.Expiry) > 50 {
			failures = append(failures, p.CheckFailure{Property: "expiry", Reason: "expiry must be at most 50 characters"})
		}
		if storeTypeKnown && args.StoreType != "cookies" {
			failures = append(
				failures,
				p.CheckFailure{Property: "expiry", Reason: "expiry is only supported for cookies"},
			)
		}
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

	body := map[string]any{
		"configIds":          []string{req.Inputs.ConfigID},
		req.Inputs.StoreType: []any{cookieConsentRulePayload(req.Inputs, false)},
	}

	var out cmpRulesListResponse
	if err := client.DoJSON(ctx, "POST", "/v1/cookie-consent/rules", nil, body, &out); err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, fmt.Errorf("create rule: %w", err)
	}
	if len(out.Items) == 0 {
		return infer.CreateResponse[CookieConsentRuleState]{}, errors.New("create rule: API returned empty items list")
	}

	created := out.Items[0]
	state, err := ruleResponseToState(req.Inputs, created)
	if err != nil {
		return infer.CreateResponse[CookieConsentRuleState]{}, fmt.Errorf("create rule response: %w", err)
	}
	return infer.CreateResponse[CookieConsentRuleState]{
		ID:     canonicalRuleID(state.ConfigID, created.RuleID),
		Output: state,
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

	configID, ruleID, _, err := parseRuleResourceID(req.ID, req.State.ConfigID)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, err
	}

	item, found, err := findCookieConsentRule(ctx, client, configID, ruleID)
	if osanoclient.IsHTTPStatus(err, http.StatusNotFound) {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{ID: ""}, nil
	}
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("read rule: %w", err)
	}
	if !found {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{ID: ""}, nil
	}

	state, err := ruleResponseToState(req.Inputs, item)
	if err != nil {
		return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{}, fmt.Errorf("read rule response: %w", err)
	}
	return infer.ReadResponse[CookieConsentRuleArgs, CookieConsentRuleState]{
		ID:     canonicalRuleID(state.ConfigID, state.RuleID),
		Inputs: state.CookieConsentRuleArgs,
		State:  state,
	}, nil
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

	_, ruleID, _, err := parseRuleResourceID(req.ID, req.State.ConfigID)
	if err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, err
	}

	var out cmpRuleResponse
	if err := client.DoJSON(
		ctx,
		http.MethodPatch,
		"/v1/cookie-consent/rules/"+strconv.Itoa(ruleID),
		nil,
		cookieConsentRulePayload(req.Inputs, true),
		&out,
	); err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, fmt.Errorf("update rule: %w", err)
	}

	state, err := ruleResponseToState(req.Inputs, out)
	if err != nil {
		return infer.UpdateResponse[CookieConsentRuleState]{}, fmt.Errorf("update rule response: %w", err)
	}
	return infer.UpdateResponse[CookieConsentRuleState]{Output: state}, nil
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
	_, ruleID, _, err := parseRuleResourceID(req.ID, req.State.ConfigID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	err = client.DoJSON(
		ctx,
		http.MethodDelete,
		"/v1/cookie-consent/rules/"+strconv.Itoa(ruleID),
		nil,
		nil,
		nil,
	)
	if osanoclient.IsHTTPStatus(err, http.StatusNotFound) {
		return infer.DeleteResponse{}, nil
	}
	if err != nil {
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
	if !ptrStringEqual(req.Inputs.RuleType, req.State.RuleType) {
		diff["ruleType"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.Description, req.State.Description) {
		diff["description"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.Expiry, req.State.Expiry) {
		diff["expiry"] = p.PropertyDiff{Kind: p.Update}
	}

	return infer.DiffResponse{
		HasChanges:   len(diff) > 0,
		DetailedDiff: diff,
	}, nil
}

func ruleResponseToState(inputs CookieConsentRuleArgs, resp cmpRuleResponse) (CookieConsentRuleState, error) {
	configID := inputs.ConfigID
	if resp.ConfigID != "" {
		configID = resp.ConfigID
	}
	storeType := inputs.StoreType
	if resp.Type != "" {
		mappedStoreType, err := ruleStoreType(resp.Type)
		if err != nil {
			return CookieConsentRuleState{}, err
		}
		storeType = mappedStoreType
	}

	return CookieConsentRuleState{
		CookieConsentRuleArgs: CookieConsentRuleArgs{
			ConfigID:       configID,
			StoreType:      storeType,
			Classification: resp.Classification,
			Rule:           resp.Rule,
			Disclosure:     resp.Disclosure,
			Title:          resp.Title,
			VendorName:     resp.VendorName,
			RuleType:       resp.RuleType,
			Description:    resp.Description,
			Expiry:         resp.Expiry,
		},
		RuleID:  resp.RuleID,
		Created: resp.Created,
		Updated: resp.Updated,
	}, nil
}

type jsonClient interface {
	DoJSON(context.Context, string, string, url.Values, any, any) error
}

func cookieConsentRulePayload(args CookieConsentRuleArgs, includeNulls bool) map[string]any {
	payload := map[string]any{
		"classification": args.Classification,
		"rule":           args.Rule,
		"disclosure":     args.Disclosure,
	}
	putNullable := func(key string, value *string) {
		if value != nil {
			payload[key] = *value
		} else if includeNulls {
			payload[key] = nil
		}
	}
	putNullable("title", args.Title)
	putNullable("vendorName", args.VendorName)
	putNullable("ruleType", args.RuleType)
	if args.StoreType == "cookies" {
		putNullable("description", args.Description)
		putNullable("expiry", args.Expiry)
	}
	return payload
}

func canonicalRuleID(configID string, ruleID int) string {
	return fmt.Sprintf("%s/%d", configID, ruleID)
}

func parseRuleResourceID(id, stateConfigID string) (configID string, ruleID int, canonicalID string, err error) {
	ruleIDText := id
	if separator := strings.LastIndex(id, "/"); separator >= 0 {
		configID = id[:separator]
		ruleIDText = id[separator+1:]
		if configID == "" {
			return "", 0, "", fmt.Errorf("invalid Cookie Consent rule ID %q: config ID is required", id)
		}
	} else {
		configID = stateConfigID
		if configID == "" {
			return "", 0, "", fmt.Errorf("invalid legacy Cookie Consent rule ID %q: configId is required in state", id)
		}
	}

	ruleID, err = strconv.Atoi(ruleIDText)
	if err != nil || ruleIDText == "" {
		return "", 0, "", fmt.Errorf("invalid Cookie Consent rule ID %q: rule ID must be an integer", id)
	}
	return configID, ruleID, canonicalRuleID(configID, ruleID), nil
}

func ruleStoreType(responseType string) (string, error) {
	switch responseType {
	case "cookie":
		return "cookies", nil
	case "script":
		return "scripts", nil
	case "iframe":
		return "iframes", nil
	case "localStorage":
		return "localStorage", nil
	default:
		return "", fmt.Errorf("unsupported Cookie Consent rule type %q", responseType)
	}
}

func findCookieConsentRule(
	ctx context.Context, client jsonClient, configID string, ruleID int,
) (cmpRuleResponse, bool, error) {
	cursor := ""
	seenCursors := map[string]bool{}
	for {
		query := url.Values{"limit": []string{"500"}}
		if cursor != "" {
			query.Set("next", cursor)
		}

		var out cmpRulesListResponse
		if err := client.DoJSON(
			ctx,
			http.MethodGet,
			"/v1/cookie-consent/configs/"+url.PathEscape(configID)+"/rules",
			query,
			nil,
			&out,
		); err != nil {
			return cmpRuleResponse{}, false, err
		}
		for idx := range out.Items {
			if out.Items[idx].RuleID == ruleID {
				return out.Items[idx], true, nil
			}
		}
		if out.Next == "" {
			return cmpRuleResponse{}, false, nil
		}
		if seenCursors[out.Next] {
			return cmpRuleResponse{}, false, fmt.Errorf("Cookie Consent rule pagination returned repeated cursor %q", out.Next)
		}
		seenCursors[out.Next] = true
		cursor = out.Next
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
