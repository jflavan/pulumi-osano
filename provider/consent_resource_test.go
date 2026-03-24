package provider

import (
	"reflect"
	"testing"
)

func TestConsentSubjectReferenceAndType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		subject ConsentSubject
		wantRef string
		wantTyp string
		wantErr string
	}{
		{
			name:    "verified ID",
			subject: ConsentSubject{VerifiedID: "verified-123"},
			wantRef: "verified-123",
			wantTyp: "subject",
		},
		{
			name:    "anonymous ID",
			subject: ConsentSubject{AnonymousID: "anon-123"},
			wantRef: "anon-123",
			wantTyp: "anonymous",
		},
		{
			name:    "missing subject",
			subject: ConsentSubject{},
			wantErr: "either subject.verifiedId or subject.anonymousId must be set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotRef, gotTyp, err := tc.subject.referenceAndType()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if gotRef != tc.wantRef || gotTyp != tc.wantTyp {
					t.Fatalf("expected %q/%q, got %q/%q", tc.wantRef, tc.wantTyp, gotRef, gotTyp)
				}
				return
			}

			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestConsentStructContractUsesReplacementAndOmitsRawResponse(t *testing.T) {
	t.Parallel()

	argsType := reflect.TypeOf(ConsentArgs{})
	for _, fieldName := range []string{
		"Subject",
		"Actions",
		"Attributes",
		"Compliance",
		"Jurisdiction",
		"Origin",
		"Tags",
	} {
		field, ok := argsType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing ConsentArgs field %q", fieldName)
		}
		if tag := field.Tag.Get("provider"); tag != "replaceOnChanges" {
			t.Fatalf("expected provider tag replaceOnChanges for %s, got %q", fieldName, tag)
		}
	}

	stateType := reflect.TypeOf(ConsentState{})
	if _, ok := stateType.FieldByName("Response"); ok {
		t.Fatal("consent state should not persist raw response output")
	}
}
