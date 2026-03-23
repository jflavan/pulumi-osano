package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateConsentArgs(t *testing.T) {
	t.Parallel()

	valid := ConsentArgs{
		Subject: ConsentSubject{VerifiedID: "verified-123"},
		Actions: []ConsentAction{
			{Target: "protocol-1", Vendor: "config-1", Action: "ACCEPT"},
		},
	}

	cases := []struct {
		name    string
		args    ConsentArgs
		wantErr string
	}{
		{
			name: "valid",
			args: valid,
		},
		{
			name:    "missing actions",
			args:    ConsentArgs{Subject: valid.Subject},
			wantErr: "at least one consent action is required",
		},
		{
			name: "missing subject",
			args: ConsentArgs{
				Actions: valid.Actions,
			},
			wantErr: "either subject.verifiedId or subject.anonymousId must be set",
		},
		{
			name: "missing target",
			args: ConsentArgs{
				Subject: valid.Subject,
				Actions: []ConsentAction{{Vendor: "config-1", Action: "ACCEPT"}},
			},
			wantErr: "actions[0].target is required",
		},
		{
			name: "privacy policy url required",
			args: ConsentArgs{
				Subject: valid.Subject,
				Actions: valid.Actions,
				Compliance: &ConsentCompliance{
					PrivacyPolicy: &ConsentPrivacyPolicy{},
				},
			},
			wantErr: "compliance.privacyPolicy.url is required when privacyPolicy is provided",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateConsentArgs(tc.args)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestResolveVerificationContact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		email       string
		phone       string
		wantChannel string
		wantValue   string
		wantErr     string
	}{
		{name: "email", email: "person@example.com", wantChannel: "email", wantValue: "person@example.com"},
		{name: "sms", phone: "+15551234567", wantChannel: "sms", wantValue: "+15551234567"},
		{name: "missing", wantErr: "either email or phone is required"},
		{
			name:    "both",
			email:   "person@example.com",
			phone:   "+15551234567",
			wantErr: "only one of email or phone can be provided",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			channel, value, err := resolveVerificationContact(tc.email, tc.phone)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if channel != tc.wantChannel || value != tc.wantValue {
					t.Fatalf("expected %q/%q, got %q/%q", tc.wantChannel, tc.wantValue, channel, value)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestSendAndVerifyPayloads(t *testing.T) {
	t.Parallel()

	sendEmail := sendCodeRequest{
		HashedSubjectID: "hashed-1",
		Channel:         "email",
		Contact:         "person@example.com",
	}
	if got := sendEmail.Payload(); !reflect.DeepEqual(got, map[string]string{
		"hashedSubjectId": "hashed-1",
		"email":           "person@example.com",
	}) {
		t.Fatalf("unexpected email payload: %#v", got)
	}

	sendSMS := sendCodeRequest{
		HashedSubjectID: "hashed-2",
		Channel:         "sms",
		Contact:         "+15551234567",
	}
	if got := sendSMS.Payload(); !reflect.DeepEqual(got, map[string]string{
		"hashedSubjectId": "hashed-2",
		"phone":           "+15551234567",
	}) {
		t.Fatalf("unexpected sms payload: %#v", got)
	}

	verifySMS := verifyRequest{
		HashedSubjectID: "hashed-3",
		Channel:         "sms",
		Contact:         "+15551234567",
		Code:            "123456",
	}
	if got := verifySMS.Payload(); !reflect.DeepEqual(got, map[string]string{
		"hashedSubjectId": "hashed-3",
		"phone":           "+15551234567",
		"code":            "123456",
	}) {
		t.Fatalf("unexpected verify payload: %#v", got)
	}
}

func TestConvertPayloadHelpers(t *testing.T) {
	t.Parallel()

	actions := convertActionsFromPayload([]unifiedConsentAction{
		{Target: "protocol-1", Vendor: "config-1", Action: "ACCEPT", Jurisdiction: "us"},
	})
	if !reflect.DeepEqual(actions, []ConsentAction{
		{Target: "protocol-1", Vendor: "config-1", Action: "ACCEPT", Jurisdiction: "us"},
	}) {
		t.Fatalf("unexpected converted actions: %#v", actions)
	}

	attrs := convertAttributesFromPayload(map[string]any{
		"count": 7,
		"flag":  true,
	})
	if !reflect.DeepEqual(attrs, map[string]string{
		"count": "7",
		"flag":  "true",
	}) {
		t.Fatalf("unexpected converted attributes: %#v", attrs)
	}
}

func TestStatusAllowedAndAPIError(t *testing.T) {
	t.Parallel()

	if !statusAllowed(http.StatusOK, nil) {
		t.Fatal("expected 200 to be allowed with no explicit allow-list")
	}
	if statusAllowed(http.StatusBadRequest, []int{http.StatusOK, http.StatusCreated}) {
		t.Fatal("expected 400 to be rejected when not explicitly allowed")
	}
	if !statusAllowed(http.StatusBadRequest, []int{http.StatusBadRequest}) {
		t.Fatal("expected 400 to be allowed when explicitly listed")
	}

	errMsg := (&apiError{StatusCode: http.StatusBadRequest, Body: "invalid subject"}).Error()
	if errMsg != "osano api request failed with status 400: invalid subject" {
		t.Fatalf("unexpected api error message: %q", errMsg)
	}
}

func TestDoJSONSendsHeadersQueryAndBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v2/test" {
			t.Fatalf("expected path /v2/test, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("ref"); got != "subject" {
			t.Fatalf("expected query ref=subject, got %q", got)
		}
		if got := r.Header.Get("x-uc-api-key"); got != "uc-key" {
			t.Fatalf("expected x-uc-api-key header, got %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "pulumi-osano/test" {
			t.Fatalf("expected custom user agent, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected JSON content-type, got %q", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &apiClient{
		settings: &apiSettings{
			baseURL:              server.URL,
			unifiedConsentAPIKey: "uc-key",
			timeout:              time.Second,
		},
		httpClient: server.Client(),
		userAgent:  "pulumi-osano/test",
	}

	body, status, err := client.doJSON(
		context.Background(),
		http.MethodPost,
		"/v2/test",
		url.Values{"ref": {"subject"}},
		map[string]string{"hello": "world"},
		headerUnifiedConsent,
		http.StatusCreated,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201 status, got %d", status)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestDoJSONRequiresConfiguredKey(t *testing.T) {
	t.Parallel()

	client := &apiClient{
		settings: &apiSettings{baseURL: "https://example.com"},
		httpClient: &http.Client{
			Timeout: time.Second,
		},
		userAgent: "pulumi-osano/test",
	}

	_, _, err := client.doJSON(
		context.Background(),
		http.MethodGet,
		"/v2/config",
		nil,
		nil,
		headerUnifiedConsent,
		http.StatusOK,
	)
	if err == nil {
		t.Fatal("expected missing-key error, got nil")
	}
	if !strings.Contains(err.Error(), "Unified Consent API key not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
