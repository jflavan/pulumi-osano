package provider

import (
	"bytes"
	"context"
	"encoding/json"
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
)

type apiClient struct {
	settings   *apiSettings
	httpClient *http.Client
	userAgent  string
}

func newAPIClient(ctx context.Context) *apiClient {
	settings := loadAPISettings(ctx)
	ua := fmt.Sprintf("pulumi-osano/%s", providerVersion)
	if strings.TrimSpace(ua) == "" {
		ua = "pulumi-osano/dev"
	}
	return &apiClient{
		settings:   settings,
		httpClient: newHTTPClient(settings.timeout),
		userAgent:  ua,
	}
}

func (c *apiClient) CreateConsent(ctx context.Context, payload consentRequestPayload) (map[string]any, error) {
	body, _, err := c.doJSON(ctx, http.MethodPost, "/v2/consents", nil, payload, headerUnifiedConsent, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode consent response: %w", err)
	}
	return data, nil
}

func (c *apiClient) FetchUnifiedConsent(ctx context.Context, subjectRef, referenceType string) (*unifiedConsentPayload, bool, error) {
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

	var data unifiedConsentPayload
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("failed to decode unified consent payload: %w", err)
	}

	return &data, true, nil
}

func (c *apiClient) FetchSubject(ctx context.Context, subjectRef, referenceType string) (*subjectPayload, bool, error) {
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

func (c *apiClient) FetchCollections(ctx context.Context, jurisdiction, collectionType string) (*collectionsPayload, error) {
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

func (c *apiClient) FetchCollection(ctx context.Context, collectionID string) (map[string]any, bool, error) {
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

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("failed to decode collection payload: %w", err)
	}

	return data, true, nil
}

func (c *apiClient) CheckConsent(ctx context.Context, subjectID string) (bool, error) {
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

func (c *apiClient) FetchConsentProfile(ctx context.Context, hashedSubjectID, configID string) (map[string]any, bool, error) {
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

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("failed to decode consent profile payload: %w", err)
	}
	return data, true, nil
}

func (c *apiClient) SendVerificationCode(ctx context.Context, req sendCodeRequest) error {
	endpoint := "/v2/subjects/send-code"
	if req.Channel == "sms" {
		endpoint = "/v2/subjects/send-code/sms"
	}

	_, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, req.Payload(), headerOsano, http.StatusOK)
	return err
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

	body, _, err := c.doJSON(ctx, http.MethodPost, endpoint, nil, req.Payload(), headerOsano, http.StatusOK)
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

type sendCodeRequest struct {
	HashedSubjectID string
	Channel         string
	Contact         string
}

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
}

func (r verifyRequest) Payload() map[string]string {
	body := map[string]string{
		"code": r.Code,
	}
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

func (c *apiClient) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	payload any,
	key headerKind,
	expectedStatus ...int,
) ([]byte, int, error) {
	base, err := url.Parse(c.settings.normalizedBaseURL())
	if err != nil {
		return nil, 0, fmt.Errorf("invalid base URL %q: %w", c.settings.baseURL, err)
	}

	rel, err := url.Parse(path)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid path %q: %w", path, err)
	}

	fullURL := base.ResolveReference(rel)
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

	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	switch key {
	case headerUnifiedConsent:
		if c.settings.unifiedConsentAPIKey == "" {
			return nil, 0, fmt.Errorf("Unified Consent API key not configured; set osano:unifiedConsentApiKey or OSANO_UC_API_KEY")
		}
		req.Header.Set("x-uc-api-key", c.settings.unifiedConsentAPIKey)
	case headerOsano:
		if c.settings.osanoAPIKey == "" {
			return nil, 0, fmt.Errorf("Osano API key not configured; set osano:osanoApiKey or OSANO_API_KEY")
		}
		req.Header.Set("x-osano-api-key", c.settings.osanoAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("Osano API request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read Osano API response: %w", err)
	}

	if !statusAllowed(resp.StatusCode, expectedStatus) {
		return nil, resp.StatusCode, &apiError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	return data, resp.StatusCode, nil
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
