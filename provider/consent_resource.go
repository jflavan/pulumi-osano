//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Consent origins and actions Osano documents for POST /v2/consents.
const (
	consentOriginAPI = "api"
	consentOriginGPC = "gpc"
)

var validConsentActions = []string{"ACCEPT", "REJECT", "UNSELECTED"}

// ConsentResource manages Osano Unified Consent submissions.
type ConsentResource struct{}

// ConsentArgs represents the inputs for creating a consent record.
type ConsentArgs struct {
	Subject             ConsentSubject     `pulumi:"subject" provider:"replaceOnChanges"`
	Compliance          *ConsentCompliance `pulumi:"compliance,optional" provider:"replaceOnChanges"`
	Actions             []ConsentAction    `pulumi:"actions,optional" provider:"replaceOnChanges"`
	Attributes          map[string]string  `pulumi:"attributes,optional" provider:"replaceOnChanges"`
	Origin              string             `pulumi:"origin,optional" provider:"replaceOnChanges"`
	Jurisdiction        string             `pulumi:"jurisdiction,optional" provider:"replaceOnChanges"`
	Tags                []string           `pulumi:"tags,optional" provider:"replaceOnChanges"`
	SessionToken        *string            `pulumi:"sessionToken,optional" provider:"secret,replaceOnChanges"`
	CountryCodeOverride *string            `pulumi:"countryCodeOverride,optional" provider:"replaceOnChanges"`
	RegionCodeOverride  *string            `pulumi:"regionCodeOverride,optional" provider:"replaceOnChanges"`
}

// ConsentState stores persisted consent metadata for an immutable consent submission.
type ConsentState struct {
	ConsentArgs
	ConsentID  string          `pulumi:"consentId"`
	LastSynced string          `pulumi:"lastSynced"`
	GPCActions []ConsentAction `pulumi:"gpcActions,optional"`
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

// Annotate documents a consent action.
func (a *ConsentAction) Annotate(an infer.Annotator) {
	an.Describe(&a.Target, "The privacy protocol ID (the Target ID on the privacy protocol's edit page).")
	an.Describe(&a.Vendor, "The Unified Consent configuration ID the consent is recorded for.")
	an.Describe(&a.Action, "The subject's choice: ACCEPT, REJECT, or UNSELECTED.")
	an.Describe(&a.Jurisdiction, "Optional jurisdiction for this action; overrides the top-level jurisdiction.")
}

// Annotate documents the consent subject.
func (s *ConsentSubject) Annotate(a infer.Annotator) {
	a.Describe(&s.VerifiedID, "The subject's verified ID. Must not contain #, %, or spaces.")
	a.Describe(&s.AnonymousID, "The subject's anonymous ID. Must not contain #, %, or spaces.")
}

// Annotate documents the compliance metadata.
func (c *ConsentCompliance) Annotate(a infer.Annotator) {
	a.Describe(&c.PrivacyPolicy, "The privacy policy in effect when the consent was given.")
	a.Describe(&c.GPC, "1 if the Global Privacy Control signal is enabled, 0 otherwise.")
}

// Annotate documents the privacy policy reference.
func (pp *ConsentPrivacyPolicy) Annotate(a infer.Annotator) {
	a.Describe(&pp.Version, "The privacy policy version active when the consent was submitted.")
	a.Describe(&pp.URL, "The privacy policy URL.")
}

// Annotate registers the Consent resource token and description.
func (r *ConsentResource) Annotate(a infer.Annotator) {
	a.SetToken("index", "Consent")
	a.Describe(
		r,
		"Submits a Unified Consent decision for a subject. Consents are immutable in Osano: changing any "+
			"input submits a new consent (replacement), and destroying the resource only removes it from "+
			"Pulumi state. Set origin to gpc and omit actions to submit a Global Privacy Control consent, "+
			"whose actions Osano derives and returns in gpcActions.",
	)
}

// Annotate documents the consent resource input schema.
func (args *ConsentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Subject, "Subject identifiers used for the consent (verifiedId or anonymousId).")
	a.Describe(&args.Compliance, "Optional compliance metadata such as the privacy policy version and GPC signal.")
	a.Describe(
		&args.Actions,
		"Consent actions referencing privacy protocols (target) within a configuration (vendor). Required "+
			"unless origin is gpc.",
	)
	a.Describe(
		&args.Attributes,
		"Optional key/value attributes stored with the consent record. Osano fills ipAddress and userAgent "+
			"itself and overwrites values sent for those keys.",
	)
	a.Describe(
		&args.Jurisdiction,
		"Optional jurisdiction, which must be one of the configuration's jurisdictions (see getCollections).",
	)
	a.Describe(
		&args.Origin,
		"Origin of the consent: api (default) or gpc. With gpc and no actions, the consent is submitted to "+
			"Osano's GPC endpoint, which derives the actions.",
	)
	a.Describe(&args.Tags, "Custom tags that Osano associates with the consent record.")
	a.Describe(&args.SessionToken, "Optional session token returned when the subject's profile was created.")
	a.Describe(
		&args.CountryCodeOverride,
		"Optional ISO 3166-1 country code Osano uses instead of resolving the caller's IP address. Set it "+
			"when submitting from a pipeline, whose IP address says nothing about the subject.",
	)
	a.Describe(
		&args.RegionCodeOverride,
		"Optional ISO 3166-2 region code Osano uses instead of resolving the caller's IP address.",
	)
}

// Annotate documents the computed consent resource state fields.
func (state *ConsentState) Annotate(a infer.Annotator) {
	a.Describe(&state.ConsentID, "Synthetic identifier used by Pulumi to track consent submissions.")
	a.Describe(&state.LastSynced, "Timestamp of the last refresh from the Osano API (RFC3339).")
	a.Describe(&state.GPCActions, "The actions Osano derived for a GPC consent submitted without actions.")
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
	_, found, err := client.FetchUnifiedConsent(ctx, subjectRef, referenceTypeSubject, req.State.geoOverride())
	if err != nil {
		return infer.ReadResponse[ConsentArgs, ConsentState]{}, err
	}
	if !found {
		return infer.ReadResponse[ConsentArgs, ConsentState]{ID: ""}, nil
	}

	// The unified consent payload merges every consent for the subject, so it cannot be mapped back to
	// this submission. Writing it into inputs would force a replacement (a duplicate consent POST) on
	// the next update, so refresh only confirms the subject still has consent and records the sync time.
	updatedState := req.State
	updatedState.LastSynced = time.Now().UTC().Format(time.RFC3339)

	return infer.ReadResponse[ConsentArgs, ConsentState]{
		ID:     req.ID,
		Inputs: req.Inputs,
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
	// Check already validated known inputs; values unknown during preview are validated on apply.
	if !dryRun {
		if err := validateConsentArgs(inputs); err != nil {
			return ConsentState{}, "", err
		}
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
	if inputs.submitsGPC() {
		actions, err := client.CreateGPCConsent(ctx, inputs.toGPCPayload(), inputs.geoOverride())
		if err != nil {
			return ConsentState{}, "", err
		}
		state.GPCActions = actions
		return state, id, nil
	}
	if _, err := client.CreateConsent(ctx, inputs.toPayload(), inputs.geoOverride()); err != nil {
		return ConsentState{}, "", err
	}
	return state, id, nil
}

// Check validates consent inputs, deferring any section that is unknown until apply.
func (r *ConsentResource) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[ConsentArgs], error) {
	args, failures, err := infer.DefaultCheck[ConsentArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ConsentArgs]{Inputs: args, Failures: failures}, err
	}

	checks := []struct {
		properties []string
		property   string
		validate   func(ConsentArgs) error
	}{
		{[]string{"actions", "origin"}, "actions", validateConsentActions},
		{[]string{"subject"}, "subject", validateConsentSubject},
		{[]string{"compliance"}, "compliance", validateConsentCompliance},
		{[]string{"origin"}, "origin", validateConsentOrigin},
	}
	for _, check := range checks {
		if anyComputed(req, check.properties...) {
			continue
		}
		if err := check.validate(args); err != nil {
			failures = append(failures, p.CheckFailure{Property: check.property, Reason: err.Error()})
		}
	}
	return infer.CheckResponse[ConsentArgs]{Inputs: args, Failures: failures}, nil
}

func anyComputed(req infer.CheckRequest, properties ...string) bool {
	for _, property := range properties {
		if req.NewInputs.Get(property).HasComputed() {
			return true
		}
	}
	return false
}

func validateConsentArgs(args ConsentArgs) error {
	for _, validate := range []func(ConsentArgs) error{
		validateConsentActions, validateConsentSubject, validateConsentCompliance, validateConsentOrigin,
	} {
		if err := validate(args); err != nil {
			return err
		}
	}
	return nil
}

func validateConsentActions(args ConsentArgs) error {
	if len(args.Actions) == 0 {
		if args.Origin == consentOriginGPC {
			return nil
		}
		return errors.New("at least one consent action is required unless origin is gpc")
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
		if err := oneOf(fmt.Sprintf("actions[%d].action", idx), action.Action, validConsentActions); err != nil {
			return err
		}
	}
	return nil
}

func validateConsentSubject(args ConsentArgs) error {
	if _, err := args.Subject.reference(); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"subject.verifiedId": args.Subject.VerifiedID, "subject.anonymousId": args.Subject.AnonymousID,
	} {
		if strings.ContainsAny(value, "#% ") {
			return fmt.Errorf("%s must not contain #, %%, or spaces", name)
		}
	}
	return nil
}

func validateConsentCompliance(args ConsentArgs) error {
	if args.Compliance == nil {
		return nil
	}
	if args.Compliance.PrivacyPolicy != nil && args.Compliance.PrivacyPolicy.URL == "" {
		return errors.New("compliance.privacyPolicy.url is required when privacyPolicy is provided")
	}
	if gpc := args.Compliance.GPC; gpc != nil && *gpc != 0 && *gpc != 1 {
		return fmt.Errorf("compliance.gpc must be 0 or 1; got %d", *gpc)
	}
	return nil
}

func validateConsentOrigin(args ConsentArgs) error {
	if args.Origin == "" {
		return nil
	}
	return oneOf("origin", args.Origin, []string{consentOriginAPI, consentOriginGPC})
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

// submitsGPC reports whether the consent goes to the GPC endpoint, which derives the actions.
func (args ConsentArgs) submitsGPC() bool {
	return args.Origin == consentOriginGPC && len(args.Actions) == 0
}

func (args ConsentArgs) geoOverride() geoOverride {
	var geo geoOverride
	if args.CountryCodeOverride != nil {
		geo.CountryCode = *args.CountryCodeOverride
	}
	if args.RegionCodeOverride != nil {
		geo.RegionCode = *args.RegionCodeOverride
	}
	return geo
}

func (args ConsentArgs) attributesPayload() map[string]string {
	// Osano requires attributes, so an unset map is sent as an empty object rather than omitted.
	if args.Attributes == nil {
		return map[string]string{}
	}
	return args.Attributes
}

func (args ConsentArgs) toPayload() consentRequestPayload {
	payload := consentRequestPayload{
		Subject:      args.Subject,
		Compliance:   args.Compliance,
		Actions:      args.Actions,
		Attributes:   args.attributesPayload(),
		Origin:       args.Origin,
		Jurisdiction: args.Jurisdiction,
		Tags:         args.Tags,
	}
	if args.SessionToken != nil {
		payload.SessionToken = *args.SessionToken
	}
	return payload
}

func (args ConsentArgs) toGPCPayload() gpcConsentRequestPayload {
	payload := gpcConsentRequestPayload{
		Subject:      args.Subject,
		Attributes:   args.attributesPayload(),
		Jurisdiction: args.Jurisdiction,
	}
	if args.Compliance != nil && args.Compliance.GPC != nil {
		payload.Compliance = &gpcCompliance{GPC: *args.Compliance.GPC}
	}
	return payload
}

type consentRequestPayload struct {
	SessionToken string             `json:"sessionToken,omitempty"`
	Subject      ConsentSubject     `json:"subject"`
	Compliance   *ConsentCompliance `json:"compliance,omitempty"`
	Actions      []ConsentAction    `json:"actions"`
	Attributes   map[string]string  `json:"attributes"`
	Origin       string             `json:"origin,omitempty"`
	Jurisdiction string             `json:"jurisdiction,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
}

type gpcCompliance struct {
	GPC int `json:"gpc"`
}

type gpcConsentRequestPayload struct {
	Subject      ConsentSubject    `json:"subject"`
	Compliance   *gpcCompliance    `json:"compliance,omitempty"`
	Attributes   map[string]string `json:"attributes"`
	Jurisdiction string            `json:"jurisdiction,omitempty"`
}

type unifiedConsentPayload struct {
	UnifiedConsent *struct {
		SubjectID        string                 `json:"subjectId"`
		BrandID          string                 `json:"brandId"`
		ChannelIDs       []string               `json:"channelIds"`
		Actions          []unifiedConsentAction `json:"actions"`
		Attributes       map[string]any         `json:"attributes"`
		Compliance       map[string]any         `json:"compliance"`
		Tags             []string               `json:"tags"`
		Jurisdiction     string                 `json:"jurisdiction"`
		LastUpdate       string                 `json:"lastUpdateDate"`
		LastConflictDate string                 `json:"lastConflictDate"`
	} `json:"unifiedConsent"`
	Conflicts []map[string]any `json:"conflicts"`
}

type unifiedConsentAction struct {
	Target       string `json:"target"`
	Vendor       string `json:"vendor"`
	Action       string `json:"action"`
	Jurisdiction string `json:"jurisdiction"`
}
