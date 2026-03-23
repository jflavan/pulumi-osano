//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ConsentResource manages Osano Unified Consent submissions.
type ConsentResource struct{}

// ConsentArgs represents the inputs for creating a consent record.
type ConsentArgs struct {
	Subject      ConsentSubject     `pulumi:"subject"`
	Compliance   *ConsentCompliance `pulumi:"compliance,optional"`
	Actions      []ConsentAction    `pulumi:"actions"`
	Attributes   map[string]string  `pulumi:"attributes,optional"`
	Origin       string             `pulumi:"origin,optional"`
	Jurisdiction string             `pulumi:"jurisdiction,optional"`
	Tags         []string           `pulumi:"tags,optional"`
}

// ConsentState stores persisted consent metadata and the latest API response.
type ConsentState struct {
	ConsentArgs
	ConsentID  string         `pulumi:"consentId"`
	LastSynced string         `pulumi:"lastSynced"`
	Response   map[string]any `pulumi:"response"`
}

// ConsentSubject identifies the subject for whom consent is recorded.
type ConsentSubject struct {
	VerifiedID  string `pulumi:"verifiedId,optional" json:"verifiedId,omitempty"`
	AnonymousID string `pulumi:"anonymousId,optional" json:"anonymousId,omitempty"`
}

// ConsentCompliance captures compliance metadata, like privacy policy details.
type ConsentCompliance struct {
	PrivacyPolicy *ConsentPrivacyPolicy `pulumi:"privacyPolicy,optional" json:"privacyPolicy,omitempty"`
	GPC           *int                  `pulumi:"gpc,optional" json:"gpc,omitempty"`
}

// ConsentPrivacyPolicy links the consent to a published privacy policy version.
type ConsentPrivacyPolicy struct {
	Version string `pulumi:"version,optional" json:"version,omitempty"`
	URL     string `pulumi:"url" json:"url"`
}

// ConsentAction declares the target (privacy protocol) and vendor (config ID).
type ConsentAction struct {
	Target       string `pulumi:"target" json:"target"`
	Vendor       string `pulumi:"vendor" json:"vendor"`
	Action       string `pulumi:"action" json:"action"`
	Jurisdiction string `pulumi:"jurisdiction,optional" json:"jurisdiction,omitempty"`
}

// Annotate registers the Consent resource token and description.
func (r *ConsentResource) Annotate(a infer.Annotator) {
	a.SetToken("index", "Consent")
	a.Describe(r, "Creates unified consent decisions within Osano for a given subject.")
}

// Annotate documents the consent resource input schema.
func (args *ConsentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Subject, "Subject identifiers used for the consent (verifiedId or anonymousId).")
	a.Describe(
		&args.Actions,
		"Consent actions referencing privacy protocols (target) within a configuration (vendor).",
	)
	a.Describe(
		&args.Attributes,
		"Optional key/value attributes stored with the consent record (e.g., ipAddress overrides).",
	)
	a.Describe(&args.Jurisdiction, "Optional jurisdiction override matching one of the configuration's jurisdictions.")
	a.Describe(&args.Origin, "Origin metadata for the consent, typically 'api' or 'gpc'.")
	a.Describe(&args.Tags, "Custom tags that Osano associates with the consent record.")
}

// Annotate documents the computed consent resource state fields.
func (state *ConsentState) Annotate(a infer.Annotator) {
	a.Describe(&state.ConsentID, "Synthetic identifier used by Pulumi to track consent submissions.")
	a.Describe(&state.LastSynced, "Timestamp of the last refresh from the Osano API (RFC3339).")
	a.Describe(&state.Response, "Latest raw response payload returned by Osano.")
}

// Diff reports when a consent resource update should submit a new consent record.
func (r *ConsentResource) Diff(
	ctx context.Context, req infer.DiffRequest[ConsentArgs, ConsentState],
) (infer.DiffResponse, error) {
	if fingerprintsEqual(req.Inputs, req.State.ConsentArgs) {
		return infer.DiffResponse{}, nil
	}

	return infer.DiffResponse{
		HasChanges: true,
		DetailedDiff: map[string]p.PropertyDiff{
			"subject":      {Kind: p.Update},
			"actions":      {Kind: p.Update},
			"attributes":   {Kind: p.Update},
			"compliance":   {Kind: p.Update},
			"jurisdiction": {Kind: p.Update},
			"origin":       {Kind: p.Update},
			"tags":         {Kind: p.Update},
		},
	}, nil
}

// Create submits a new consent record to Osano.
func (r *ConsentResource) Create(
	ctx context.Context, req infer.CreateRequest[ConsentArgs],
) (infer.CreateResponse[ConsentState], error) {
	state, id, err := applyConsent(ctx, req.Inputs, "", req.DryRun)
	if err != nil {
		return infer.CreateResponse[ConsentState]{}, err
	}

	return infer.CreateResponse[ConsentState]{
		ID:     id,
		Output: state,
	}, nil
}

// Read refreshes the local state from the upstream unified consent payload.
func (r *ConsentResource) Read(
	ctx context.Context, req infer.ReadRequest[ConsentArgs, ConsentState],
) (infer.ReadResponse[ConsentArgs, ConsentState], error) {
	subjectRef, err := req.State.Subject.reference()
	if err != nil {
		return infer.ReadResponse[ConsentArgs, ConsentState]{}, err
	}

	client := newAPIClient(ctx)
	payload, found, err := client.FetchUnifiedConsent(ctx, subjectRef, "subject")
	if err != nil {
		return infer.ReadResponse[ConsentArgs, ConsentState]{}, err
	}
	if !found {
		return infer.ReadResponse[ConsentArgs, ConsentState]{ID: ""}, nil
	}

	updatedState := req.State
	updatedState.LastSynced = time.Now().UTC().Format(time.RFC3339)
	updatedState.Response = map[string]any{
		"unifiedConsent": payload.UnifiedConsent,
		"conflicts":      payload.Conflicts,
	}

	if payload.UnifiedConsent != nil {
		updatedState.Actions = convertActionsFromPayload(payload.UnifiedConsent.Actions)
		updatedState.Attributes = convertAttributesFromPayload(payload.UnifiedConsent.Attributes)
	}

	return infer.ReadResponse[ConsentArgs, ConsentState]{
		ID:     req.ID,
		Inputs: updatedState.ConsentArgs,
		State:  updatedState,
	}, nil
}

// Update submits a replacement consent record for the tracked subject state.
func (r *ConsentResource) Update(
	ctx context.Context, req infer.UpdateRequest[ConsentArgs, ConsentState],
) (infer.UpdateResponse[ConsentState], error) {
	state, _, err := applyConsent(ctx, req.Inputs, req.ID, req.DryRun)
	if err != nil {
		return infer.UpdateResponse[ConsentState]{}, err
	}
	return infer.UpdateResponse[ConsentState]{
		Output: state,
	}, nil
}

// Delete forgets the local Pulumi resource without deleting upstream consent history.
func (r *ConsentResource) Delete(context.Context, infer.DeleteRequest[ConsentState]) (infer.DeleteResponse, error) {
	// Osano consents are immutable historical records. Destroying the Pulumi resource
	// simply forgets the local tracking without attempting to delete upstream data.
	return infer.DeleteResponse{}, nil
}

func applyConsent(
	ctx context.Context,
	inputs ConsentArgs,
	existingID string,
	dryRun bool,
) (ConsentState, string, error) {
	if err := validateConsentArgs(inputs); err != nil {
		return ConsentState{}, "", err
	}

	id := existingID
	if id == "" {
		id = "consent-" + uuid.NewString()
	}

	state := ConsentState{
		ConsentArgs: inputs,
		ConsentID:   id,
		LastSynced:  time.Now().UTC().Format(time.RFC3339),
		Response:    map[string]any{},
	}

	if dryRun {
		return state, id, nil
	}

	client := newAPIClient(ctx)
	payload := inputs.toPayload()
	resp, err := client.CreateConsent(ctx, payload)
	if err != nil {
		return ConsentState{}, "", err
	}
	state.Response = resp
	return state, id, nil
}

func validateConsentArgs(args ConsentArgs) error {
	if len(args.Actions) == 0 {
		return errors.New("at least one consent action is required")
	}
	for idx, action := range args.Actions {
		if action.Target == "" {
			return fmt.Errorf("actions[%d].target is required", idx)
		}
		if action.Vendor == "" {
			return fmt.Errorf("actions[%d].vendor is required", idx)
		}
		if action.Action == "" {
			return fmt.Errorf("actions[%d].action is required", idx)
		}
	}

	if _, err := args.Subject.reference(); err != nil {
		return err
	}

	if args.Compliance != nil && args.Compliance.PrivacyPolicy != nil {
		if args.Compliance.PrivacyPolicy.URL == "" {
			return errors.New("compliance.privacyPolicy.url is required when privacyPolicy is provided")
		}
	}

	return nil
}

func (s ConsentSubject) reference() (string, error) {
	if s.VerifiedID != "" {
		return s.VerifiedID, nil
	}
	if s.AnonymousID != "" {
		return s.AnonymousID, nil
	}
	return "", errors.New("either subject.verifiedId or subject.anonymousId must be set")
}

func (args ConsentArgs) toPayload() consentRequestPayload {
	return consentRequestPayload{
		Subject:      args.Subject,
		Compliance:   args.Compliance,
		Actions:      args.Actions,
		Attributes:   args.Attributes,
		Origin:       args.Origin,
		Jurisdiction: args.Jurisdiction,
		Tags:         args.Tags,
	}
}

func fingerprintsEqual(a, b ConsentArgs) bool {
	return reflect.DeepEqual(a, b)
}

func convertActionsFromPayload(actions []unifiedConsentAction) []ConsentAction {
	if len(actions) == 0 {
		return nil
	}
	converted := make([]ConsentAction, 0, len(actions))
	for _, action := range actions {
		converted = append(converted, ConsentAction{
			Target:       action.Target,
			Vendor:       action.Vendor,
			Action:       action.Action,
			Jurisdiction: action.Jurisdiction,
		})
	}
	return converted
}

func convertAttributesFromPayload(attrs map[string]any) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for k, v := range attrs {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

type consentRequestPayload struct {
	Subject      ConsentSubject     `json:"subject"`
	Compliance   *ConsentCompliance `json:"compliance,omitempty"`
	Actions      []ConsentAction    `json:"actions"`
	Attributes   map[string]string  `json:"attributes,omitempty"`
	Origin       string             `json:"origin,omitempty"`
	Jurisdiction string             `json:"jurisdiction,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
}

type unifiedConsentPayload struct {
	UnifiedConsent *struct {
		SubjectID    string                 `json:"subjectId"`
		Actions      []unifiedConsentAction `json:"actions"`
		Attributes   map[string]any         `json:"attributes"`
		Compliance   map[string]any         `json:"compliance"`
		Tags         []string               `json:"tags"`
		Jurisdiction string                 `json:"jurisdiction"`
		LastUpdate   string                 `json:"lastUpdateDate"`
	} `json:"unifiedConsent"`
	Conflicts []map[string]any `json:"conflicts"`
}

type unifiedConsentAction struct {
	Target       string `json:"target"`
	Vendor       string `json:"vendor"`
	Action       string `json:"action"`
	Jurisdiction string `json:"jurisdiction"`
}
