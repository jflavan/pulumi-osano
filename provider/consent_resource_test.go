package provider

import (
	"reflect"
	"strings"
	"testing"
)

func TestConsentSubjectReference(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		subject ConsentSubject
		wantRef string
		wantErr string
	}{
		{name: "verified ID", subject: ConsentSubject{VerifiedID: "verified-123"}, wantRef: "verified-123"},
		{name: "anonymous ID", subject: ConsentSubject{AnonymousID: "anon-123"}, wantRef: "anon-123"},
		{
			name:    "verified ID wins over anonymous ID",
			subject: ConsentSubject{VerifiedID: "verified-123", AnonymousID: "anon-123"},
			wantRef: "verified-123",
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

			gotRef, err := tc.subject.reference()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if gotRef != tc.wantRef {
					t.Fatalf("expected %q, got %q", tc.wantRef, gotRef)
				}
				return
			}

			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNormalizeReferenceType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ input, want string }{
		{"", "subject"}, {"subject", "subject"}, {" session ", "session"}, {"anonymous", "subject"},
	} {
		got, err := normalizeReferenceType(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("normalizeReferenceType(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	if _, err := normalizeReferenceType("verified"); err == nil {
		t.Fatal("expected an unknown reference type to fail")
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
	// Consents are immutable, so every input, including the newer optional ones, must replace.
	for idx := range argsType.NumField() {
		field := argsType.Field(idx)
		if !strings.Contains(field.Tag.Get("provider"), "replaceOnChanges") {
			t.Fatalf("expected ConsentArgs.%s to be replaceOnChanges", field.Name)
		}
	}
	if tag, _ := argsType.FieldByName("SessionToken"); !strings.Contains(tag.Tag.Get("provider"), "secret") {
		t.Fatal("expected sessionToken to be secret")
	}

	stateType := reflect.TypeOf(ConsentState{})
	if _, ok := stateType.FieldByName("Response"); ok {
		t.Fatal("consent state should not persist raw response output")
	}
}
