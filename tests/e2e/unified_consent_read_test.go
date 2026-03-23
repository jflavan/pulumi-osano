//go:build e2e && consentread

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jflavan/pulumi-osano/tests/e2e/internal/api"
	"github.com/jflavan/pulumi-osano/tests/e2e/internal/testenv"
)

func TestUnifiedConsentLookups(t *testing.T) {
	testenv.RequireOptIn(t, testenv.EnvRunConsentE2E, "enable unified consent read tests")

	client, err := api.NewClientFromEnv(false)
	if err != nil {
		t.Fatalf("unable to initialize API client: %v", err)
	}

	subjectRef := testenv.Require(t, testenv.EnvTestSubjectRef, "subject reference value")
	referenceType := testenv.Optional(testenv.EnvTestReferenceType, "subject")
	configID := testenv.Require(t, testenv.EnvTestConfigID, "configuration identifier")
	hashedSubjectID := testenv.Require(t, testenv.EnvTestHashedSubjectID, "hashed subject identifier")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	payload, found, err := client.FetchUnifiedConsent(ctx, subjectRef, referenceType)
	if err != nil {
		t.Fatalf("fetch unified consent failed: %v", err)
	}
	if !found {
		t.Fatalf("no unified consent returned for %s (%s)", subjectRef, referenceType)
	}
	if payload.UnifiedConsent == nil {
		t.Fatalf("unified consent payload missing subject data")
	}

	subject, found, err := client.FetchSubject(ctx, subjectRef, referenceType)
	if err != nil {
		t.Fatalf("fetch subject failed: %v", err)
	}
	if !found {
		t.Fatalf("subject %s (%s) not found", subjectRef, referenceType)
	}
	if subject.ID == "" {
		t.Fatalf("subject response missing id")
	}

	exists, err := client.CheckConsent(ctx, subject.ID)
	if err != nil {
		t.Fatalf("check consent failed: %v", err)
	}
	if !exists {
		t.Fatalf("consent not reported for subject %s", subject.ID)
	}

	profile, found, err := client.FetchConsentProfile(ctx, hashedSubjectID, configID)
	if err != nil {
		t.Fatalf("fetch consent profile failed: %v", err)
	}
	if !found {
		t.Fatalf("consent profile not found for hashed subject %s", hashedSubjectID)
	}
	if len(profile) == 0 {
		t.Fatalf("consent profile payload empty")
	}
}

func TestConfigAndCollectionsReads(t *testing.T) {
	testenv.RequireOptIn(t, testenv.EnvRunConsentE2E, "enable unified consent read tests")

	client, err := api.NewClientFromEnv(false)
	if err != nil {
		t.Fatalf("unable to initialize API client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	config, err := client.FetchConfig(ctx)
	if err != nil {
		t.Fatalf("fetch config failed: %v", err)
	}
	if len(config) == 0 {
		t.Fatalf("config payload was empty")
	}

	jurisdiction := testenv.Optional(testenv.EnvTestCollectionsJurisdiction, "")
	collectionType := testenv.Optional(testenv.EnvTestCollectionsType, "")
	collections, err := client.FetchCollections(ctx, jurisdiction, collectionType)
	if err != nil {
		t.Fatalf("fetch collections failed: %v", err)
	}
	if len(collections.Jurisdictions) == 0 {
		t.Fatalf("collections response missing jurisdictions")
	}
	if len(collections.Collection) == 0 {
		t.Fatalf("collections response missing collection detail")
	}

	collectionID := testenv.Require(t, testenv.EnvTestCollectionID, "collection identifier")
	collection, found, err := client.FetchCollection(ctx, collectionID)
	if err != nil {
		t.Fatalf("fetch collection failed: %v", err)
	}
	if !found {
		t.Fatalf("collection %s not found", collectionID)
	}
	if len(collection) == 0 {
		t.Fatalf("collection %s payload empty", collectionID)
	}

	t.Logf("config %s contained %d jurisdictions; collection %s returned %d keys", describeConfigID(config), len(collections.Jurisdictions), collectionID, len(collection))
}

func describeConfigID(config map[string]any) string {
	if v, ok := config["id"].(string); ok && v != "" {
		return v
	}
	if v, ok := config["name"].(string); ok && v != "" {
		return v
	}
	return fmt.Sprintf("payload:%d-keys", len(config))
}
