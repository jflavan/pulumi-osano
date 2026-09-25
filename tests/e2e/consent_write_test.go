//go:build e2e && consentwrite

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jflavan/pulumi-osano/tests/e2e/internal/api"
	"github.com/jflavan/pulumi-osano/tests/e2e/internal/testenv"
)

func TestCreateConsentRecord(t *testing.T) {
	testenv.RequireOptIn(t, testenv.EnvRunWriteE2E, "enable consent write tests")

	client, err := api.NewClientFromEnv(false)
	if err != nil {
		t.Fatalf("unable to initialize API client: %v", err)
	}

	action := testenv.Require(t, testenv.EnvWriteAction, "consent action (e.g. optIn)")
	privacyProtocolID := testenv.Require(t, testenv.EnvWritePrivacyProtocolID, "privacy protocol identifier")
	configID := testenv.Require(t, testenv.EnvTestConfigID, "configuration identifier for vendor field")
	subject := buildConsentSubject(t)
	subjectRef, referenceType := resolveSubjectRef(subject)
	if subjectRef == "" {
		t.Fatalf("subject reference could not be resolved")
	}

	traceID := uuid.NewString()
	payload := api.ConsentRequestPayload{
		Subject: subject,
		Actions: []api.ConsentAction{
			{
				Target: privacyProtocolID,
				Vendor: configID,
				Action: action,
			},
		},
		Attributes: map[string]string{
			"pulumiTraceId": traceID,
		},
		Origin: "api",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := client.CreateConsent(ctx, payload); err != nil {
		t.Fatalf("create consent failed: %v", err)
	}

	readback, found, err := client.FetchUnifiedConsent(ctx, subjectRef, referenceType)
	if err != nil {
		t.Fatalf("fetch unified consent failed: %v", err)
	}
	if !found || readback.UnifiedConsent == nil {
		t.Fatalf("unified consent state missing after write")
	}

	attrs := readback.UnifiedConsent.Attributes
	if attrs == nil {
		t.Fatalf("unified consent state missing attributes for verification")
	}
	value, ok := attrs["pulumiTraceId"]
	if !ok || fmt.Sprintf("%v", value) != traceID {
		t.Fatalf("expected pulumiTraceId attribute %s, got %v", traceID, value)
	}
}

func buildConsentSubject(t *testing.T) api.ConsentSubject {
	t.Helper()
	subjectType := strings.ToLower(testenv.Require(t, testenv.EnvWriteSubjectType, "subject type (verified|anonymous)"))
	subjectValue := testenv.Require(t, testenv.EnvWriteSubjectValue, "subject identifier value")

	switch subjectType {
	case "verified", "verifiedid", "subject":
		return api.ConsentSubject{VerifiedID: subjectValue}
	case "anonymous", "anonymousid":
		return api.ConsentSubject{AnonymousID: subjectValue}
	default:
		t.Fatalf("unsupported subject type %q", subjectType)
		return api.ConsentSubject{}
	}
}

// resolveSubjectRef returns the reference to read the subject back with. Osano's ref parameter only
// accepts subject and session: verified and anonymous IDs are both subject references.
func resolveSubjectRef(subject api.ConsentSubject) (reference, referenceType string) {
	if subject.VerifiedID != "" {
		return subject.VerifiedID, "subject"
	}
	if subject.AnonymousID != "" {
		return subject.AnonymousID, "subject"
	}
	return "", ""
}
