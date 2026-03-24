//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// GetUnifiedConsent exposes an invoke to fetch the latest unified consent summary for a subject.
type GetUnifiedConsent struct{}

// GetUnifiedConsentArgs captures the lookup parameters for the invoke.
type GetUnifiedConsentArgs struct {
	SubjectRef    string `pulumi:"subjectRef"`
	ReferenceType string `pulumi:"referenceType,optional"`
}

// Annotate documents the getUnifiedConsent input fields.
func (args *GetUnifiedConsentArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for getUnifiedConsent")
}

// GetUnifiedConsentResult is returned to Pulumi programs.
type GetUnifiedConsentResult struct {
	SubjectRef     string           `pulumi:"subjectRef"`
	Exists         bool             `pulumi:"exists"`
	UnifiedConsent map[string]any   `pulumi:"unifiedConsent"`
	Conflicts      []map[string]any `pulumi:"conflicts"`
}

// Annotate registers the getUnifiedConsent invoke schema metadata.
func (g *GetUnifiedConsent) Annotate(a infer.Annotator) {
	a.SetToken("index", "getUnifiedConsent")
	a.Describe(g, "Fetches the unified consent state for a subject reference using the Unified Consent API key.")
}

// Invoke fetches unified consent state for the requested subject reference.
func (g *GetUnifiedConsent) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetUnifiedConsentArgs],
) (infer.FunctionResponse[GetUnifiedConsentResult], error) {
	if req.Input.SubjectRef == "" {
		return infer.FunctionResponse[GetUnifiedConsentResult]{}, errors.New("subjectRef is required")
	}

	client := newAPIClient(ctx)
	payload, found, err := client.FetchUnifiedConsent(ctx, req.Input.SubjectRef, req.Input.ReferenceType)
	if err != nil {
		return infer.FunctionResponse[GetUnifiedConsentResult]{}, err
	}
	if !found {
		return infer.FunctionResponse[GetUnifiedConsentResult]{
			Output: GetUnifiedConsentResult{
				SubjectRef: req.Input.SubjectRef,
				Exists:     false,
			},
		}, nil
	}

	result := GetUnifiedConsentResult{
		SubjectRef: req.Input.SubjectRef,
		Exists:     true,
		Conflicts:  payload.Conflicts,
	}

	if payload.UnifiedConsent != nil {
		result.UnifiedConsent = map[string]any{
			"subjectId":      payload.UnifiedConsent.SubjectID,
			"jurisdiction":   payload.UnifiedConsent.Jurisdiction,
			"lastUpdateDate": payload.UnifiedConsent.LastUpdate,
			"actions":        convertActionMaps(payload.UnifiedConsent.Actions),
			"attributes":     payload.UnifiedConsent.Attributes,
			"compliance":     payload.UnifiedConsent.Compliance,
			"tags":           payload.UnifiedConsent.Tags,
		}
	}

	return infer.FunctionResponse[GetUnifiedConsentResult]{Output: result}, nil
}

func convertActionMaps(actions []unifiedConsentAction) []map[string]any {
	if len(actions) == 0 {
		return nil
	}
	converted := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		converted = append(converted, map[string]any{
			"target":       action.Target,
			"vendor":       action.Vendor,
			"action":       action.Action,
			"jurisdiction": action.Jurisdiction,
		})
	}
	return converted
}

// GetSubject exposes subject lookup for unified consent references.
type GetSubject struct{}

// GetSubjectArgs captures the subject lookup parameters.
type GetSubjectArgs struct {
	SubjectRef    string `pulumi:"subjectRef"`
	ReferenceType string `pulumi:"referenceType,optional"`
}

// GetSubjectResult provides the ID pairings for a subject reference.
type GetSubjectResult struct {
	SubjectRef  string `pulumi:"subjectRef"`
	SubjectID   string `pulumi:"subjectId"`
	VerifiedID  string `pulumi:"verifiedId"`
	AnonymousID string `pulumi:"anonymousId"`
	Exists      bool   `pulumi:"exists"`
}

// Annotate registers the getSubject invoke schema metadata.
func (g *GetSubject) Annotate(a infer.Annotator) {
	a.SetToken("index", "getSubject")
	a.Describe(g, "Fetches subject identifiers (subject, verified, anonymous IDs) for a reference or session ID.")
}

// Invoke resolves subject identifiers for the requested subject reference.
func (g *GetSubject) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetSubjectArgs],
) (infer.FunctionResponse[GetSubjectResult], error) {
	if strings.TrimSpace(req.Input.SubjectRef) == "" {
		return infer.FunctionResponse[GetSubjectResult]{}, errors.New("subjectRef is required")
	}

	client := newAPIClient(ctx)
	payload, found, err := client.FetchSubject(ctx, req.Input.SubjectRef, req.Input.ReferenceType)
	if err != nil {
		return infer.FunctionResponse[GetSubjectResult]{}, err
	}

	if !found {
		return infer.FunctionResponse[GetSubjectResult]{
			Output: GetSubjectResult{
				SubjectRef: req.Input.SubjectRef,
				Exists:     false,
			},
		}, nil
	}

	return infer.FunctionResponse[GetSubjectResult]{
		Output: GetSubjectResult{
			SubjectRef:  req.Input.SubjectRef,
			SubjectID:   payload.ID,
			VerifiedID:  payload.VerifiedID,
			AnonymousID: payload.AnonymousID,
			Exists:      true,
		},
	}, nil
}

// GetConfig returns the UC configuration associated with the Unified Consent API key.
type GetConfig struct{}

// GetConfigArgs holds the arguments for getConfig.
type GetConfigArgs struct{}

// GetConfigResult returns the Unified Consent configuration payload.
type GetConfigResult struct {
	Config map[string]any `pulumi:"config"`
}

// Annotate registers the getConfig invoke schema metadata.
func (g *GetConfig) Annotate(a infer.Annotator) {
	a.SetToken("index", "getConfig")
	a.Describe(g, "Retrieves the Unified Consent configuration referenced by the API key.")
}

// Invoke retrieves the Unified Consent configuration payload.
func (g *GetConfig) Invoke(
	ctx context.Context,
	_ infer.FunctionRequest[GetConfigArgs],
) (infer.FunctionResponse[GetConfigResult], error) {
	client := newAPIClient(ctx)
	config, err := client.FetchConfig(ctx)
	if err != nil {
		return infer.FunctionResponse[GetConfigResult]{}, err
	}

	return infer.FunctionResponse[GetConfigResult]{
		Output: GetConfigResult{Config: config},
	}, nil
}

// GetCollections lists privacy protocol collections for the configured UC tenant.
type GetCollections struct{}

// GetCollectionsArgs holds optional collection filters.
type GetCollectionsArgs struct {
	Jurisdiction string `pulumi:"jurisdiction,optional"`
	Type         string `pulumi:"type,optional"`
}

// Annotate documents the getCollections input fields.
func (args *GetCollectionsArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for getCollections")
}

// GetCollectionsResult returns the collection aggregate payload.
type GetCollectionsResult struct {
	Jurisdictions []string       `pulumi:"jurisdictions"`
	Collection    map[string]any `pulumi:"collection"`
}

// Annotate registers the getCollections invoke schema metadata.
func (g *GetCollections) Annotate(a infer.Annotator) {
	a.SetToken("index", "getCollections")
	a.Describe(
		g,
		"Retrieves the aggregated privacy protocol collections for an optional jurisdiction "+
			"and version (published/draft).",
	)
}

// Invoke lists privacy protocol collections for the configured tenant.
func (g *GetCollections) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetCollectionsArgs],
) (infer.FunctionResponse[GetCollectionsResult], error) {
	client := newAPIClient(ctx)
	payload, err := client.FetchCollections(ctx, req.Input.Jurisdiction, req.Input.Type)
	if err != nil {
		return infer.FunctionResponse[GetCollectionsResult]{}, err
	}

	return infer.FunctionResponse[GetCollectionsResult]{
		Output: GetCollectionsResult{
			Jurisdictions: payload.Jurisdictions,
			Collection:    payload.Collection,
		},
	}, nil
}

// GetCollection fetches a single privacy protocol collection by ID.
type GetCollection struct{}

// GetCollectionArgs identifies the collection to fetch.
type GetCollectionArgs struct {
	CollectionID string `pulumi:"collectionId"`
}

// Annotate documents the getCollection input fields.
func (args *GetCollectionArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for getCollection")
}

// GetCollectionResult returns a single collection lookup result.
type GetCollectionResult struct {
	CollectionID string         `pulumi:"collectionId"`
	Collection   map[string]any `pulumi:"collection"`
	Exists       bool           `pulumi:"exists"`
}

// Annotate registers the getCollection invoke schema metadata.
func (g *GetCollection) Annotate(a infer.Annotator) {
	a.SetToken("index", "getCollection")
	a.Describe(g, "Retrieves a specific privacy protocol collection by ID.")
}

// Invoke fetches a collection by ID.
func (g *GetCollection) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetCollectionArgs],
) (infer.FunctionResponse[GetCollectionResult], error) {
	if strings.TrimSpace(req.Input.CollectionID) == "" {
		return infer.FunctionResponse[GetCollectionResult]{}, errors.New("collectionId is required")
	}

	client := newAPIClient(ctx)
	payload, found, err := client.FetchCollection(ctx, req.Input.CollectionID)
	if err != nil {
		return infer.FunctionResponse[GetCollectionResult]{}, err
	}

	if !found {
		return infer.FunctionResponse[GetCollectionResult]{
			Output: GetCollectionResult{
				CollectionID: req.Input.CollectionID,
				Exists:       false,
			},
		}, nil
	}

	return infer.FunctionResponse[GetCollectionResult]{
		Output: GetCollectionResult{
			CollectionID: req.Input.CollectionID,
			Collection:   payload,
			Exists:       true,
		},
	}, nil
}

// CheckConsent reports whether unified consent exists for a subject ID.
type CheckConsent struct{}

// CheckConsentArgs identifies the subject to inspect.
type CheckConsentArgs struct {
	SubjectID string `pulumi:"subjectId"`
}

// Annotate documents the checkConsent input fields.
func (args *CheckConsentArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for checkConsent")
}

// CheckConsentResult reports whether consent exists for a subject.
type CheckConsentResult struct {
	SubjectID string `pulumi:"subjectId"`
	Exists    bool   `pulumi:"exists"`
}

// Annotate registers the checkConsent invoke schema metadata.
func (c *CheckConsent) Annotate(a infer.Annotator) {
	a.SetToken("index", "checkConsent")
	a.Describe(c, "Checks whether a unified consent record exists for a given subject ID.")
}

// Invoke checks whether a consent record exists for the subject.
func (c *CheckConsent) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[CheckConsentArgs],
) (infer.FunctionResponse[CheckConsentResult], error) {
	subjectID := strings.TrimSpace(req.Input.SubjectID)
	if subjectID == "" {
		return infer.FunctionResponse[CheckConsentResult]{}, errors.New("subjectId is required")
	}

	client := newAPIClient(ctx)
	exists, err := client.CheckConsent(ctx, subjectID)
	if err != nil {
		return infer.FunctionResponse[CheckConsentResult]{}, err
	}

	return infer.FunctionResponse[CheckConsentResult]{
		Output: CheckConsentResult{
			SubjectID: subjectID,
			Exists:    exists,
		},
	}, nil
}

// GetConsentProfile fetches hashed consent profile data.
type GetConsentProfile struct{}

// GetConsentProfileArgs identifies the consent profile to retrieve.
type GetConsentProfileArgs struct {
	HashedSubjectID string `pulumi:"hashedSubjectId"`
	ConfigID        string `pulumi:"configId"`
}

// Annotate documents the getConsentProfile input fields.
func (args *GetConsentProfileArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for getConsentProfile")
}

// GetConsentProfileResult returns a consent profile lookup result.
type GetConsentProfileResult struct {
	HashedSubjectID string         `pulumi:"hashedSubjectId"`
	ConfigID        string         `pulumi:"configId"`
	Profile         map[string]any `pulumi:"profile"`
	Exists          bool           `pulumi:"exists"`
}

// Annotate registers the getConsentProfile invoke schema metadata.
func (g *GetConsentProfile) Annotate(a infer.Annotator) {
	a.SetToken("index", "getConsentProfile")
	a.Describe(g, "Retrieves a consent profile for a hashed subject identifier and config ID.")
}

// Invoke fetches a consent profile for the supplied hashed subject ID and config ID.
func (g *GetConsentProfile) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetConsentProfileArgs],
) (infer.FunctionResponse[GetConsentProfileResult], error) {
	hashed := strings.TrimSpace(req.Input.HashedSubjectID)
	configID := strings.TrimSpace(req.Input.ConfigID)
	if hashed == "" {
		return infer.FunctionResponse[GetConsentProfileResult]{}, errors.New("hashedSubjectId is required")
	}
	if configID == "" {
		return infer.FunctionResponse[GetConsentProfileResult]{}, errors.New("configId is required")
	}

	client := newAPIClient(ctx)
	profile, found, err := client.FetchConsentProfile(ctx, hashed, configID)
	if err != nil {
		return infer.FunctionResponse[GetConsentProfileResult]{}, err
	}

	result := GetConsentProfileResult{
		HashedSubjectID: hashed,
		ConfigID:        configID,
		Exists:          found,
	}
	if found {
		result.Profile = profile
	}

	return infer.FunctionResponse[GetConsentProfileResult]{Output: result}, nil
}

// SendSubjectCode starts the verification flow for a subject profile.
type SendSubjectCode struct{}

// SendSubjectCodeArgs identifies the subject and delivery channel for verification.
type SendSubjectCodeArgs struct {
	HashedSubjectID string `pulumi:"hashedSubjectId"`
	Email           string `pulumi:"email,optional"`
	Phone           string `pulumi:"phone,optional"`
}

// Annotate documents the sendSubjectCode input fields.
func (args *SendSubjectCodeArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for sendSubjectCode")
}

// SendSubjectCodeResult reports where the verification code was sent.
type SendSubjectCodeResult struct {
	HashedSubjectID string `pulumi:"hashedSubjectId"`
	Channel         string `pulumi:"channel"`
	Destination     string `pulumi:"destination"`
}

// Annotate registers the sendSubjectCode invoke schema metadata.
func (s *SendSubjectCode) Annotate(a infer.Annotator) {
	a.SetToken("index", "sendSubjectCode")
	a.Describe(s, "Sends a verification code to a subject's email or phone using the Osano API key.")
}

// Invoke sends a verification code to the requested subject contact.
func (s *SendSubjectCode) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[SendSubjectCodeArgs],
) (infer.FunctionResponse[SendSubjectCodeResult], error) {
	hashed := strings.TrimSpace(req.Input.HashedSubjectID)
	if hashed == "" {
		return infer.FunctionResponse[SendSubjectCodeResult]{}, errors.New("hashedSubjectId is required")
	}
	channel, destination, err := resolveVerificationContact(req.Input.Email, req.Input.Phone)
	if err != nil {
		return infer.FunctionResponse[SendSubjectCodeResult]{}, err
	}

	client := newAPIClient(ctx)
	if err := client.SendVerificationCode(ctx, sendCodeRequest{
		HashedSubjectID: hashed,
		Channel:         channel,
		Contact:         destination,
	}); err != nil {
		return infer.FunctionResponse[SendSubjectCodeResult]{}, err
	}

	return infer.FunctionResponse[SendSubjectCodeResult]{
		Output: SendSubjectCodeResult{
			HashedSubjectID: hashed,
			Channel:         channel,
			Destination:     destination,
		},
	}, nil
}

// VerifySubjectCode finalizes a subject verification challenge.
type VerifySubjectCode struct{}

// VerifySubjectCodeArgs captures the contact and code used for verification.
type VerifySubjectCodeArgs struct {
	HashedSubjectID string `pulumi:"hashedSubjectId"`
	Code            string `pulumi:"code"`
	Email           string `pulumi:"email,optional"`
	Phone           string `pulumi:"phone,optional"`
}

// Annotate documents the verifySubjectCode input fields.
func (args *VerifySubjectCodeArgs) Annotate(a infer.Annotator) {
	a.Describe(args, "Arguments for verifySubjectCode")
}

// VerifySubjectCodeResult reports whether verification succeeded and returns the profile.
type VerifySubjectCodeResult struct {
	HashedSubjectID string         `pulumi:"hashedSubjectId"`
	Channel         string         `pulumi:"channel"`
	Destination     string         `pulumi:"destination"`
	Verified        bool           `pulumi:"verified"`
	Profile         map[string]any `pulumi:"profile"`
}

// Annotate registers the verifySubjectCode invoke schema metadata.
func (v *VerifySubjectCode) Annotate(a infer.Annotator) {
	a.SetToken("index", "verifySubjectCode")
	a.Describe(v, "Verifies a subject profile using the code sent via email or SMS.")
}

// Invoke verifies the subject profile with the supplied code and contact.
func (v *VerifySubjectCode) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[VerifySubjectCodeArgs],
) (infer.FunctionResponse[VerifySubjectCodeResult], error) {
	hashed := strings.TrimSpace(req.Input.HashedSubjectID)
	if hashed == "" {
		return infer.FunctionResponse[VerifySubjectCodeResult]{}, errors.New("hashedSubjectId is required")
	}
	code := strings.TrimSpace(req.Input.Code)
	if code == "" {
		return infer.FunctionResponse[VerifySubjectCodeResult]{}, errors.New("code is required")
	}
	channel, destination, err := resolveVerificationContact(req.Input.Email, req.Input.Phone)
	if err != nil {
		return infer.FunctionResponse[VerifySubjectCodeResult]{}, err
	}

	client := newAPIClient(ctx)
	profile, err := client.VerifySubjectCode(ctx, verifyRequest{
		HashedSubjectID: hashed,
		Channel:         channel,
		Contact:         destination,
		Code:            code,
	})
	if err != nil {
		return infer.FunctionResponse[VerifySubjectCodeResult]{}, err
	}

	result := VerifySubjectCodeResult{
		HashedSubjectID: hashed,
		Channel:         channel,
		Destination:     destination,
		Verified:        true,
	}
	if len(profile) > 0 {
		result.Profile = profile
	}

	return infer.FunctionResponse[VerifySubjectCodeResult]{Output: result}, nil
}

func resolveVerificationContact(email, phone string) (channel, destination string, err error) {
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	if email == "" && phone == "" {
		return "", "", errors.New("either email or phone is required")
	}
	if email != "" && phone != "" {
		return "", "", errors.New("only one of email or phone can be provided")
	}
	if email != "" {
		return "email", email, nil
	}
	return "sms", phone, nil
}
