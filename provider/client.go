//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type headerKind int

const (
	headerUnifiedConsent headerKind = iota
	headerOsano
	// headerSubject authenticates subject-verification routes with the Osano API key and falls back
	// to the Unified Consent API key. Osano's guide requires the Osano key for routes that create or
	// verify subjects, while its OpenAPI spec lists the Unified Consent key for these routes.
	headerSubject
)

// Unified Consent reference types accepted by the ref query parameter.
const (
	referenceTypeSubject = "subject"
	referenceTypeSession = "session"
	// referenceTypeAnonymous was never an Osano reference type: anonymous IDs are subject references.
	// It is accepted as a deprecated alias of subject so existing programs keep working.
	referenceTypeAnonymous = "anonymous"
)

// normalizeReferenceType maps a user-supplied reference type to the ref value Osano accepts.
func normalizeReferenceType(referenceType string) (string, error) {
	switch strings.TrimSpace(referenceType) {
	case "", referenceTypeSubject, referenceTypeAnonymous:
		return referenceTypeSubject, nil
	case referenceTypeSession:
		return referenceTypeSession, nil
	default:
		return "", fmt.Errorf(
			"referenceType must be subject or session (anonymous is a deprecated alias of subject); got %q",
			referenceType,
		)
	}
}

// geoOverride carries the optional country and region Osano uses instead of resolving the caller's
// IP address, which in a pipeline is the CI runner's address rather than the subject's.
type geoOverride struct {
	CountryCode string
	RegionCode  string
}

func (g geoOverride) headers() http.Header {
	headers := http.Header{}
	if code := strings.TrimSpace(g.CountryCode); code != "" {
		headers.Set("x-country-code-override", code)
	}
	if code := strings.TrimSpace(g.RegionCode); code != "" {
		headers.Set("x-region-code-override", code)
	}
	return headers
}

type apiClient struct {
	settings   *apiSettings
	httpClient *http.Client
	userAgent  string
}

func newAPIClient(ctx context.Context) *apiClient {
	settings := loadAPISettings(ctx)
	return &apiClient{
		settings:   settings,
		httpClient: newHTTPClient(settings.timeout),
		userAgent:  providerUserAgent(),
	}
}

// providerUserAgent identifies this provider and its version to the Osano APIs, for example
// pulumi-osano/0.1.0.
func providerUserAgent() string {
	return userAgentForVersion(providerVersion)
}

// userAgentForVersion builds the provider user agent from a build version. GoReleaser stamps the
// git tag (v0.1.0) and `make provider` the bare version (0.1.0), so drop a leading "v" to send the
// same pulumi-osano/0.1.0 either way.
func userAgentForVersion(version string) string {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return "pulumi-osano/dev"
	}
	return "pulumi-osano/" + version
}

func (c *apiClient) CreateConsent(
	ctx context.Context, payload consentRequestPayload, geo geoOverride,
) (map[string]any, error) {
	body, _, err := c.doJSONWithHeaders(
		ctx, http.MethodPost, "/v2/consents", nil, payload, geo.headers(), headerUnifiedConsent, http.StatusCreated,
	)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode consent response: %w", err)
	}
	return data, nil
}

// CreateGPCConsent submits a Global Privacy Control consent. Osano derives the actions from the
// configuration's privacy protocols and the subject's jurisdiction, and returns them.
func (c *apiClient) CreateGPCConsent(
	ctx context.Context, payload gpcConsentRequestPayload, geo geoOverride,
) ([]ConsentAction, error) {
	body, _, err := c.doJSONWithHeaders(
		ctx, http.MethodPost, "/v2/consents/gpc", nil, payload, geo.headers(), headerUnifiedConsent, http.StatusCreated,
	)
	if err != nil {
		return nil, err
	}

	var data struct {
		GPCActions []ConsentAction `json:"gpcActions"`
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to decode GPC consent response: %w", err)
		}
	}
	return data.GPCActions, nil
}

func (c *apiClient) FetchUnifiedConsent(
	ctx context.Context,
	subjectRef, referenceType string,
	geo geoOverride,
) (*unifiedConsentPayload, bool, error) {
	ref, err := normalizeReferenceType(referenceType)
	if err != nil {
		return nil, false, err
	}

	query := url.Values{}
	query.Set("ref", ref)

	path := "/v2/consents/unified/" + url.PathEscape(subjectRef)
	body, status, err := c.doJSONWithHeaders(
		ctx,
		http.MethodGet,
		path,
		query,
		nil,
		geo.headers(),
		headerUnifiedConsent,
		http.StatusOK,
		http.StatusBadRequest,
	)
	if err != nil {
		return nil, false, err
	}

	// Osano answers 400 when the subject has no consents.
	if status == http.StatusBadRequest {
		return nil, false, nil
	}

	var data unifiedConsentPayload
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("failed to decode unified consent payload: %w", err)
	}

	return &data, true, nil
}

func (c *apiClient) FetchSubject(ctx context.Context, subjectRef, referenceType string) (*subjectPayload, bool, error) {
	ref, err := normalizeReferenceType(referenceType)
	if err != nil {
		return nil, false, err
	}

	query := url.Values{}
	query.Set("ref", ref)

	path := "/v2/subjects/" + url.PathEscape(subjectRef)
	body, status, err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		query,
		nil,
		headerUnifiedConsent,
		http.StatusOK,
		http.StatusBadRequest,
		http.StatusNotFound,
	)
	if err != nil {
		return nil, false, err
	}

	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, false, nil
	}

	var data subjectPayload
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("failed to decode subject payload: %w", err)
	}

	return &data, true, nil
}

func (c *apiClient) FetchConfig(ctx context.Context) (map[string]any, error) {
	body, _, err := c.doJSON(ctx, http.MethodGet, "/v2/config", nil, nil, headerUnifiedConsent, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode config payload: %w", err)
	}
	return data, nil
}

func (c *apiClient) FetchCollections(
	ctx context.Context,
	jurisdiction, collectionType string,
) (*collectionsPayload, error) {
	query := url.Values{}
	if trimmed := strings.TrimSpace(jurisdiction); trimmed != "" {
		query.Set("jurisdiction", trimmed)
	}
	if trimmed := strings.TrimSpace(collectionType); trimmed != "" {
		query.Set("type", trimmed)
	}

	body, _, err := c.doJSON(ctx, http.MethodGet, "/v2/collections", query, nil, headerUnifiedConsent, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var data collectionsPayload
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode collections payload: %w", err)
	}
	return &data, nil
}

func (c *apiClient) FetchCollection(
	ctx context.Context,
	collectionID string,
) (collection map[string]any, found bool, err error) {
	path := "/v2/collections/" + url.PathEscape(collectionID)
	body, status, err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		nil,
		headerUnifiedConsent,
		http.StatusOK,
		http.StatusNotFound,
	)
	if err != nil {
		return nil, false, err
	}

	if status == http.StatusNotFound {
		return nil, false, nil
	}

	if err := json.Unmarshal(body, &collection); err != nil {
		return nil, false, fmt.Errorf("failed to decode collection payload: %w", err)
	}

	return collection, true, nil
}

func (c *apiClient) CheckConsent(ctx context.Context, subjectID string, geo geoOverride) (bool, error) {
	path := "/v2/consents/check/" + url.PathEscape(subjectID)
	body, _, err := c.doJSONWithHeaders(
		ctx, http.MethodGet, path, nil, nil, geo.headers(), headerUnifiedConsent, http.StatusOK,
	)
	if err != nil {
		return false, err
	}

	var resp struct {
		Exists bool `json:"exists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("failed to decode consent check payload: %w", err)
	}
	return resp.Exists, nil
}

func (c *apiClient) FetchConsentProfile(
	ctx context.Context,
	hashedSubjectID, configID string,
	geo geoOverride,
) (profile map[string]any, found bool, err error) {
	query := url.Values{}
	query.Set("configId", configID)
	path := "/v2/consent-profiles/" + url.PathEscape(hashedSubjectID)
	body, status, err := c.doJSONWithHeaders(
		ctx,
		http.MethodGet,
		path,
		query,
		nil,
		geo.headers(),
		headerUnifiedConsent,
		http.StatusOK,
		http.StatusBadRequest,
		http.StatusNotFound,
	)
	if err != nil {
		return nil, false, err
	}

	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, false, nil
	}

	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, false, fmt.Errorf("failed to decode consent profile payload: %w", err)
	}
	return profile, true, nil
}

// SendVerificationCode sends a one-time code and returns the decoded response body. Osano does not
// document the response; for SMS it may carry the session that verification requires.
func (c *apiClient) SendVerificationCode(ctx context.Context, req sendCodeRequest) (map[string]any, error) {
	endpoint := "/v2/subjects/send-code"
	if req.Channel == "sms" {
		endpoint = "/v2/subjects/send-code/sms"
	}

	body, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, req.Payload(), headerSubject, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		// The response is undocumented, so a body that is not a JSON object is ignored.
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, nil
		}
	}
	return data, nil
}

func (c *apiClient) VerifySubjectCode(ctx context.Context, req verifyRequest) (map[string]any, error) {
	var endpoint string
	switch req.Channel {
	case "email":
		endpoint = "/v2/subjects/profile/verify"
	case "sms":
		endpoint = "/v2/subjects/profile/verify/sms"
	default:
		return nil, fmt.Errorf("unsupported verification channel: %s", req.Channel)
	}

	body, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, req.Payload(), headerSubject, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to decode verification payload: %w", err)
		}
	}
	return data, nil
}

// FetchSubjectProfile reads the profile (email and subject ID) of a subject.
func (c *apiClient) FetchSubjectProfile(
	ctx context.Context, subjectID string,
) (profile map[string]any, found bool, err error) {
	return c.fetchOptionalObject(ctx, "/v2/subjects/"+url.PathEscape(subjectID)+"/profile", "subject profile")
}

// FetchSession reads the subject and profile associated with a session ID.
func (c *apiClient) FetchSession(
	ctx context.Context, sessionID string,
) (session map[string]any, found bool, err error) {
	return c.fetchOptionalObject(ctx, "/v2/sessions/"+url.PathEscape(sessionID), "session")
}

// fetchOptionalObject GETs a JSON object with the Unified Consent key, reporting 400 and 404 as
// not found.
func (c *apiClient) fetchOptionalObject(
	ctx context.Context, path, what string,
) (object map[string]any, found bool, err error) {
	body, status, err := c.doJSON(
		ctx, http.MethodGet, path, nil, nil, headerUnifiedConsent,
		http.StatusOK, http.StatusBadRequest, http.StatusNotFound,
	)
	if err != nil {
		return nil, false, err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, false, nil
	}

	var data map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, false, fmt.Errorf("failed to decode %s payload: %w", what, err)
		}
	}
	return data, true, nil
}

type sendCodeRequest struct {
	HashedSubjectID string
	Channel         string
	Contact         string
}

// Payload builds the send-code body. Osano documents only email or phone; hashedSubjectId is sent
// only when the caller supplies it.
func (r sendCodeRequest) Payload() map[string]string {
	body := map[string]string{}
	if r.HashedSubjectID != "" {
		body["hashedSubjectId"] = r.HashedSubjectID
	}
	switch r.Channel {
	case "email":
		body["email"] = r.Contact
	case "sms":
		body["phone"] = r.Contact
	}
	return body
}

type verifyRequest struct {
	HashedSubjectID string
	Channel         string
	Contact         string
	Code            string
	Session         string
}

// Payload builds the verify body. SMS verification also requires the session of the SMS challenge.
func (r verifyRequest) Payload() map[string]string {
	body := map[string]string{
		"code": r.Code,
	}
	if r.HashedSubjectID != "" {
		body["hashedSubjectId"] = r.HashedSubjectID
	}
	if r.Session != "" {
		body["session"] = r.Session
	}
	switch r.Channel {
	case "email":
		body["email"] = r.Contact
	case "sms":
		body["phone"] = r.Contact
	}
	return body
}

func (c *apiClient) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	payload any,
	key headerKind,
	expectedStatus ...int,
) (respBody []byte, status int, err error) {
	return c.doJSONWithHeaders(ctx, method, path, query, payload, nil, key, expectedStatus...)
}

// doJSONWithHeaders is doJSON with extra request headers, such as geolocation overrides.
func (c *apiClient) doJSONWithHeaders(
	ctx context.Context,
	method, path string,
	query url.Values,
	payload any,
	headers http.Header,
	key headerKind,
	expectedStatus ...int,
) (respBody []byte, status int, err error) {
	base := c.settings.normalizedBaseURL()
	if _, err := url.Parse(base); err != nil {
		return nil, 0, fmt.Errorf("invalid base URL %q: %w", c.settings.baseURL, err)
	}

	// Append rather than resolve so a base URL path prefix (for example a proxy mount) is kept.
	fullURL, err := url.Parse(base + path)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid path %q: %w", path, err)
	}
	if query != nil {
		q := fullURL.Query()
		for key, values := range query {
			for _, v := range values {
				q.Add(key, v)
			}
		}
		fullURL.RawQuery = q.Encode()
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to encode request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	switch key {
	case headerUnifiedConsent:
		if c.settings.unifiedConsentAPIKey == "" {
			return nil, 0, errors.New(
				"Unified Consent API key not configured; set osano:unifiedConsentApiKey or OSANO_UC_API_KEY",
			)
		}
		req.Header.Set("x-uc-api-key", c.settings.unifiedConsentAPIKey)
	case headerOsano:
		if c.settings.osanoAPIKey == "" {
			return nil, 0, errors.New(
				"Osano API key not configured; set osano:osanoApiKey or OSANO_API_KEY",
			)
		}
		req.Header.Set("x-osano-api-key", c.settings.osanoAPIKey)
	case headerSubject:
		switch {
		case c.settings.osanoAPIKey != "":
			req.Header.Set("x-osano-api-key", c.settings.osanoAPIKey)
		case c.settings.unifiedConsentAPIKey != "":
			req.Header.Set("x-uc-api-key", c.settings.unifiedConsentAPIKey)
		default:
			return nil, 0, errors.New(
				"no Osano API key configured; set osano:osanoApiKey or OSANO_API_KEY " +
					"(or osano:unifiedConsentApiKey or OSANO_UC_API_KEY)",
			)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("Osano API request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close Osano API response: %w", closeErr)
		}
	}()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read Osano API response: %w", err)
	}

	if !statusAllowed(resp.StatusCode, expectedStatus) {
		return nil, resp.StatusCode, &apiError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}

	return respBody, resp.StatusCode, nil
}

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("osano api request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("osano api request failed with status %d: %s", e.StatusCode, e.Body)
}

func statusAllowed(status int, allowed []int) bool {
	if len(allowed) == 0 {
		return status >= 200 && status < 300
	}
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

type subjectPayload struct {
	ID          string `json:"id"`
	VerifiedID  string `json:"verifiedId"`
	AnonymousID string `json:"anonymousId"`
}

type collectionsPayload struct {
	Jurisdictions []string       `json:"jurisdictions"`
	Collection    map[string]any `json:"collection"`
}
