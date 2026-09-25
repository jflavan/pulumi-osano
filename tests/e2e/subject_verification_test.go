//go:build e2e && subjectverification

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jflavan/pulumi-osano/tests/e2e/internal/api"
	"github.com/jflavan/pulumi-osano/tests/e2e/internal/testenv"
)

func TestSendAndVerifySubjectCode(t *testing.T) {
	testenv.RequireOptIn(t, testenv.EnvRunSubjectE2E, "enable subject verification tests")

	client, err := api.NewClientFromEnv(true)
	if err != nil {
		t.Fatalf("unable to initialize API client: %v", err)
	}

	// Osano's current API identifies the subject by email or phone; a hashed subject ID is optional.
	hashedSubjectID := testenv.Optional(testenv.EnvHashedSubjectID, "")
	channel := strings.ToLower(testenv.Require(t, testenv.EnvVerificationChannel, "verification channel (email or sms)"))
	contact := resolveVerificationContact(t, channel)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Logf("Sending verification code via %s to %s", channel, redactContact(contact))
	sent, err := client.SendVerificationCode(ctx, channel, contact, hashedSubjectID)
	if err != nil {
		t.Fatalf("send verification code failed: %v", err)
	}
	t.Logf("send-code response keys: %v", mapKeys(sent))

	// SMS verification requires the challenge session. Osano does not document where it comes from,
	// so take it from the send-code response when present, or from the environment.
	session := testenv.Optional(testenv.EnvVerificationSession, "")
	for _, key := range []string{"session", "sessionId"} {
		if value, ok := sent[key].(string); ok && value != "" && session == "" {
			session = value
		}
	}
	if channel == "sms" && session == "" {
		t.Fatalf("SMS verification needs a session: the send-code response had none; set %s",
			testenv.EnvVerificationSession)
	}

	code := strings.TrimSpace(os.Getenv(testenv.EnvVerificationCode))
	if code == "" {
		var readErr error
		code, readErr = promptForVerificationCode()
		if readErr != nil {
			t.Fatalf("unable to capture verification code: %v", readErr)
		}
	}
	if code == "" {
		t.Fatalf("verification code cannot be empty")
	}

	profile, err := client.VerifySubjectCode(ctx, channel, contact, hashedSubjectID, code, session)
	if err != nil {
		t.Fatalf("verify subject code failed: %v", err)
	}
	if len(profile) == 0 {
		t.Fatalf("verification succeeded but profile payload was empty")
	}

	t.Logf("Verification succeeded; profile keys: %v", mapKeys(profile))
}

func resolveVerificationContact(t *testing.T, channel string) string {
	t.Helper()
	switch channel {
	case "email":
		return testenv.Require(t, testenv.EnvVerificationEmail, "email recipient for verification code")
	case "sms":
		return testenv.Require(t, testenv.EnvVerificationPhone, "phone recipient for verification code")
	default:
		t.Fatalf("unsupported verification channel %q", channel)
		return ""
	}
}

func promptForVerificationCode() (string, error) {
	fmt.Print("Enter the verification code you received via email/SMS: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	return strings.TrimSpace(code), err
}

func redactContact(contact string) string {
	trimmed := strings.TrimSpace(contact)
	if len(trimmed) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-4:]
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
