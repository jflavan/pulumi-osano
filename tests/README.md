# Tests

## Provider tests

Go provider tests live under `provider/`. Run them with:

```bash
make test_provider
```

Whenever you modify code inside `provider/`, remember to run `make codegen` afterwards so the schema
and SDKs stay in sync.

## End-to-end tests

Opt-in integration tests now live under [tests/e2e](tests/e2e). Shared helpers in
[tests/e2e/internal/testenv](tests/e2e/internal/testenv/env.go) centralize the environment variables
each suite needs. Every test file is guarded by a build tag so you only run the flows you have
credentials for:

- Subject verification: `-tags "e2e subjectverification"`
- Unified consent reads: `-tags "e2e consentread"`
- Consent writes: `-tags "e2e consentwrite"`

In addition to the build tag, each suite checks for an opt-in flag before hitting the live API:

| Suite | Opt-in env | Description |
| --- | --- | --- |
| Subject verification | `OSANO_RUN_SUBJECT_E2E` | Drives send/verify subject code invokes |
| Consent reads | `OSANO_RUN_CONSENT_E2E` | Covers unified consent, subjects, profiles, config, and collections |
| Consent writes | `OSANO_RUN_WRITE_E2E` | Submits a real consent record and validates read-back |

The GitHub Actions acceptance workflow now runs the read-only suite automatically when the required
repository secrets are configured. Write and subject-verification suites remain opt-in/manual because
they either mutate live data or require a one-time verification code.

### Subject verification flow

Test location: [tests/e2e/subject_verification_test.go](tests/e2e/subject_verification_test.go)

Required environment:

1. API keys: `OSANO_UC_API_KEY` and `OSANO_API_KEY`
2. Subject context: `OSANO_HASHED_SUBJECT_ID`
3. Delivery channel: `OSANO_VERIFICATION_CHANNEL` ("email" or "sms") and the matching contact:
   `OSANO_VERIFICATION_EMAIL` or `OSANO_VERIFICATION_PHONE`
4. Optional: `OSANO_VERIFICATION_CODE` if you already know the code (otherwise the test prompts)
5. Opt-in: `OSANO_RUN_SUBJECT_E2E=1`

Run:

```bash
go test ./tests/e2e -run TestSendAndVerifySubjectCode -tags "e2e subjectverification"
```

The test sends a fresh verification code every time. Ensure the target inbox/device is under your
control and monitor it while the test runs.

### Unified consent read suite

Test location: [tests/e2e/unified_consent_read_test.go](tests/e2e/unified_consent_read_test.go)

Required environment:

1. API key: `OSANO_UC_API_KEY`
2. Subject lookup: `OSANO_TEST_SUBJECT_REF` and optional `OSANO_TEST_REFERENCE_TYPE` (defaults to
   `subject`)
3. Consent profile: `OSANO_TEST_CONFIG_ID` and `OSANO_TEST_HASHED_SUBJECT_ID`
4. Collections: `OSANO_TEST_COLLECTION_ID` plus optional filters `OSANO_TEST_COLLECTIONS_JURISDICTION`
   and `OSANO_TEST_COLLECTIONS_TYPE`
5. Opt-in: `OSANO_RUN_CONSENT_E2E=1`

Run:

```bash
go test ./tests/e2e -run "TestUnifiedConsentLookups|TestConfigAndCollectionsReads" -tags "e2e consentread"
```

The first test exercises unified consent, subject lookup, consent existence checks, and consent
profile reads. The second verifies config, filtered collections, and a direct collection lookup.

### Consent write path

Test location: [tests/e2e/consent_write_test.go](tests/e2e/consent_write_test.go)

Required environment:

1. API key: `OSANO_UC_API_KEY`
2. Subject definition: `OSANO_WRITE_SUBJECT_TYPE` ("verified" or "anonymous") and
   `OSANO_WRITE_SUBJECT_VALUE`
3. Action context: `OSANO_TEST_CONFIG_ID`, `OSANO_WRITE_PRIVACY_PROTOCOL_ID`, and `OSANO_WRITE_ACTION`
4. Opt-in: `OSANO_RUN_WRITE_E2E=1`

Run:

```bash
go test ./tests/e2e -run TestCreateConsentRecord -tags "e2e consentwrite"
```

Each run records a consent with a unique trace attribute, then immediately fetches the unified
consent state to verify the attribute was persisted.

If you introduce new resources or invokes, add mocked HTTP unit tests inside `provider/` first, then
extend the relevant E2E file (or add a new one under `tests/e2e`) once you're ready to exercise the
real API.
