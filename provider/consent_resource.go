//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// ConsentResource manages Osano Unified Consent submissions.
type ConsentResource struct{}

// ConsentArgs represents the inputs for creating a consent record.
type ConsentArgs struct {
	Subject      ConsentSubject     `pulumi:"subject" provider:"replaceOnChanges"`
	Compliance   *ConsentCompliance `pulumi:"compliance,optional" provider:"replaceOnChanges"`
	Actions      []ConsentAction    `pulumi:"actions" provider:"replaceOnChanges"`
	Attributes   map[string]string  `pulumi:"attributes,optional" provider:"replaceOnChanges"`
	Origin       string             `pulumi:"origin,optional" provider:"replaceOnChanges"`
	Jurisdiction string             `pulumi:"jurisdiction,optional" provider:"replaceOnChanges"`
	Tags         []string           `pulumi:"tags,optional" provider:"replaceOnChanges"`
}

// ConsentState stores persisted consent metadata for an immutable consent submission.
type ConsentState struct {
	ConsentArgs
	ConsentID  string `pulumi:"consentId"`
	LastSynced string `pulumi:"lastSynced"`
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
	subjectRef, referenceType, err := req.State.Subject.referenceAndType()
	if err != nil {
		return infer.ReadResponse[ConsentArgs, ConsentState]{}, err
	}

	client := newAPIClient(ctx)
	payload, found, err := client.FetchUnifiedConsent(ctx, subjectRef, referenceType)
	if err != nil {
		return infer.ReadResponse[ConsentArgs, ConsentState]{}, err
	}
	if !found {
		return infer.ReadResponse[ConsentArgs, ConsentState]{ID: ""}, nil
	}

	updatedState := req.State
	updatedState.LastSynced = time.Now().UTC().Format(time.RFC3339)

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
	}

	if dryRun {
		return state, id, nil
	}

	client := newAPIClient(ctx)
	payload := inputs.toPayload()
	if _, err := client.CreateConsent(ctx, payload); err != nil {
		return ConsentState{}, "", err
	}
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
	ref, _, err := s.referenceAndType()
	return ref, err
}

func (s ConsentSubject) referenceAndType() (reference, referenceType string, err error) {
	if s.VerifiedID != "" {
		return s.VerifiedID, "subject", nil
	}
	if s.AnonymousID != "" {
		return s.AnonymousID, "anonymous", nil
	}
	return "", "", errors.New("either subject.verifiedId or subject.anonymousId must be set")
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
