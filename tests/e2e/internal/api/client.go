package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jflavan/pulumi-osano/tests/e2e/internal/testenv"
)

const (
	defaultBaseURL = "https://uc.api.osano.com"
	defaultTimeout = 60 * time.Second
)

type headerKind int

const (
	headerUnifiedConsent headerKind = iota
	headerOsano
)

// Client is a thin HTTP helper for Osano endpoints used in E2E tests.
type Client struct {
	baseURL    string
	ucAPIKey   string
	osanoKey   string
	httpClient *http.Client
	userAgent  string
}

// NewClientFromEnv builds a client using the standard Osano environment variables.
func NewClientFromEnv(requireOsanoKey bool) (*Client, error) {
	baseURL := strings.TrimSpace(testenv.MaybeGet(testenv.EnvAPIBaseURL))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	ucKey := strings.TrimSpace(testenv.MaybeGet(testenv.EnvUnifiedConsentAPIKey))
	if ucKey == "" {
		return nil, fmt.Errorf("set %s with a Unified Consent API key", testenv.EnvUnifiedConsentAPIKey)
	}
	osanoKey := strings.TrimSpace(testenv.MaybeGet(testenv.EnvOsanoAPIKey))
	if requireOsanoKey && osanoKey == "" {
		return nil, fmt.Errorf("set %s with an Osano API key", testenv.EnvOsanoAPIKey)
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		ucAPIKey:   ucKey,
		osanoKey:   osanoKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		userAgent:  "pulumi-osano-e2e-tests",
	}, nil
}

// UnifiedConsentPayload captures the subset of fields used by the tests.
type UnifiedConsentPayload struct {
	UnifiedConsent *struct {
		SubjectID  string            `json:"subjectId"`
		Actions    []map[string]any  `json:"actions"`
		Attributes map[string]string `json:"attributes"`
	} `json:"unifiedConsent"`
	Conflicts []map[string]any `json:"conflicts"`
}

// SubjectPayload captures identifiers returned by Osano.
type SubjectPayload struct {
	ID          string `json:"id"`
	VerifiedID  string `json:"verifiedId"`
	AnonymousID string `json:"anonymousId"`
}

// CollectionsPayload represents the aggregated collections response.
type CollectionsPayload struct {
	Jurisdictions []string       `json:"jurisdictions"`
	Collection    map[string]any `json:"collection"`
}

// ConsentRequestPayload mirrors the provider's consent submission body.
type ConsentRequestPayload struct {
	Subject      ConsentSubject     `json:"subject"`
	Compliance   *ConsentCompliance `json:"compliance,omitempty"`
	Actions      []ConsentAction    `json:"actions"`
	Attributes   map[string]string  `json:"attributes,omitempty"`
	Origin       string             `json:"origin,omitempty"`
	Jurisdiction string             `json:"jurisdiction,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
}

// ConsentSubject identifies a subject by verified or anonymous ID.
type ConsentSubject struct {
	VerifiedID  string `json:"verifiedId,omitempty"`
	AnonymousID string `json:"anonymousId,omitempty"`
}

// ConsentCompliance optionally links the record to a privacy policy and GPC state.
type ConsentCompliance struct {
	PrivacyPolicy *ConsentPrivacyPolicy `json:"privacyPolicy,omitempty"`
	GPC           *int                  `json:"gpc,omitempty"`
}

// ConsentPrivacyPolicy ties the consent to a published policy version.
type ConsentPrivacyPolicy struct {
	Version string `json:"version,omitempty"`
	URL     string `json:"url"`
}

// ConsentAction represents a single privacy protocol decision.
type ConsentAction struct {
	Target       string `json:"target"`
	Vendor       string `json:"vendor"`
	Action       string `json:"action"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
}

func (c *Client) CreateConsent(ctx context.Context, payload ConsentRequestPayload) (map[string]any, error) {
	if len(payload.Actions) == 0 {
		return nil, fmt.Errorf("at least one consent action is required")
	}
	body, _, err := c.doJSON(ctx, http.MethodPost, "/v2/consents", nil, payload, headerUnifiedConsent, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode consent response: %w", err)
	}
	return resp, nil
}

func (c *Client) FetchUnifiedConsent(ctx context.Context, subjectRef, referenceType string) (*UnifiedConsentPayload, bool, error) {
	if referenceType == "" {
		referenceType = "subject"
	}
	query := url.Values{}
	query.Set("ref", referenceType)
	path := fmt.Sprintf("/v2/consents/unified/%s", url.PathEscape(subjectRef))
	body, status, err := c.doJSON(ctx, http.MethodGet, path, query, nil, headerUnifiedConsent, http.StatusOK, http.StatusBadRequest)
	if err != nil {
		if apiErr, ok := err.(*apiError); ok && apiErr.StatusCode == http.StatusBadRequest {
			return nil, false, nil
		}
		return nil, false, err
	}
	if status == http.StatusBadRequest {
		return nil, false, nil
	}
	var payload UnifiedConsentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("failed to decode unified consent payload: %w", err)
	}
	return &payload, true, nil
}

func (c *Client) FetchSubject(ctx context.Context, subjectRef, referenceType string) (*SubjectPayload, bool, error) {
	if referenceType == "" {
		referenceType = "subject"
	}
	query := url.Values{}
	query.Set("ref", referenceType)
	path := fmt.Sprintf("/v2/subjects/%s", url.PathEscape(subjectRef))
	body, status, err := c.doJSON(ctx, http.MethodGet, path, query, nil, headerUnifiedConsent, http.StatusOK, http.StatusBadRequest, http.StatusNotFound)
	if err != nil {
		if apiErr, ok := err.(*apiError); ok && (apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, false, nil
	}
	var payload SubjectPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("failed to decode subject payload: %w", err)
	}
	return &payload, true, nil
}

func (c *Client) CheckConsent(ctx context.Context, subjectID string) (bool, error) {
	path := fmt.Sprintf("/v2/consents/check/%s", url.PathEscape(subjectID))
	body, _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, headerUnifiedConsent, http.StatusOK)
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

func (c *Client) FetchConsentProfile(ctx context.Context, hashedSubjectID, configID string) (map[string]any, bool, error) {
	query := url.Values{}
	query.Set("configId", configID)
	path := fmt.Sprintf("/v2/consent-profiles/%s", url.PathEscape(hashedSubjectID))
	body, status, err := c.doJSON(ctx, http.MethodGet, path, query, nil, headerUnifiedConsent, http.StatusOK, http.StatusBadRequest, http.StatusNotFound)
	if err != nil {
		if apiErr, ok := err.(*apiError); ok && (apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return nil, false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("failed to decode consent profile payload: %w", err)
	}
	return payload, true, nil
}

func (c *Client) FetchConfig(ctx context.Context) (map[string]any, error) {
	body, _, err := c.doJSON(ctx, http.MethodGet, "/v2/config", nil, nil, headerUnifiedConsent, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode config payload: %w", err)
	}
	return payload, nil
}

func (c *Client) FetchCollections(ctx context.Context, jurisdiction, collectionType string) (*CollectionsPayload, error) {
	query := url.Values{}
	if v := strings.TrimSpace(jurisdiction); v != "" {
		query.Set("jurisdiction", v)
	}
	if v := strings.TrimSpace(collectionType); v != "" {
		query.Set("type", v)
	}
	body, _, err := c.doJSON(ctx, http.MethodGet, "/v2/collections", query, nil, headerUnifiedConsent, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var payload CollectionsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode collections payload: %w", err)
	}
	return &payload, nil
}

func (c *Client) FetchCollection(ctx context.Context, collectionID string) (map[string]any, bool, error) {
	path := fmt.Sprintf("/v2/collections/%s", url.PathEscape(collectionID))
	body, status, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, headerUnifiedConsent, http.StatusOK, http.StatusNotFound)
	if err != nil {
		if apiErr, ok := err.(*apiError); ok && apiErr.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if status == http.StatusNotFound {
		return nil, false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("failed to decode collection payload: %w", err)
	}
	return payload, true, nil
}

func (c *Client) SendVerificationCode(ctx context.Context, channel, contact, hashedSubjectID string) error {
	if c.osanoKey == "" {
		return fmt.Errorf("Osano API key is required to send verification codes; set %s", testenv.EnvOsanoAPIKey)
	}
	payload := map[string]string{"hashedSubjectId": hashedSubjectID}
	endpoint := "/v2/subjects/send-code"
	switch strings.ToLower(channel) {
	case "email":
		payload["email"] = contact
	case "sms":
		endpoint = "/v2/subjects/send-code/sms"
		payload["phone"] = contact
	default:
		return fmt.Errorf("unsupported verification channel %q", channel)
	}
	_, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, payload, headerOsano, http.StatusOK)
	return err
}

func (c *Client) VerifySubjectCode(ctx context.Context, channel, contact, hashedSubjectID, code string) (map[string]any, error) {
	if c.osanoKey == "" {
		return nil, fmt.Errorf("Osano API key is required to verify subject codes; set %s", testenv.EnvOsanoAPIKey)
	}
	payload := map[string]string{
		"hashedSubjectId": hashedSubjectID,
		"code":            code,
	}
	var endpoint string
	switch strings.ToLower(channel) {
	case "email":
		payload["email"] = contact
		endpoint = "/v2/subjects/profile/verify"
	case "sms":
		payload["phone"] = contact
		endpoint = "/v2/subjects/profile/verify/sms"
	default:
		return nil, fmt.Errorf("unsupported verification channel %q", channel)
	}
	body, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, payload, headerOsano, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to decode verification payload: %w", err)
		}
	}
	return data, nil
}

type apiError struct {
	StatusCode int
	body       []byte
}

func (e *apiError) Error() string {
	return fmt.Sprintf("osano api request failed with status %d: %s", e.StatusCode, string(e.body))
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	payload any,
	header headerKind,
	expectedStatus ...int,
) ([]byte, int, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid base URL %q: %w", c.baseURL, err)
	}
	rel, err := url.Parse(path)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid path %q: %w", path, err)
	}
	fullURL := base.ResolveReference(rel)
	if query != nil {
		q := fullURL.Query()
		for k, values := range query {
			for _, v := range values {
				q.Add(k, v)
			}
		}
		fullURL.RawQuery = q.Encode()
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to encode request payload: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build HTTP request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch header {
	case headerUnifiedConsent:
		req.Header.Set("x-uc-api-key", c.ucAPIKey)
	case headerOsano:
		req.Header.Set("x-osano-api-key", c.osanoKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("osano api request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}
	for _, status := range expectedStatus {
		if resp.StatusCode == status {
			return data, resp.StatusCode, nil
		}
	}
	return nil, resp.StatusCode, &apiError{StatusCode: resp.StatusCode, body: data}
}
