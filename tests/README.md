# Tests

## Provider tests

Go provider tests live under `provider/`. Run them with:

```bash
make test_provider
```

Whenever you modify code inside `provider/`, remember to run `make codegen` afterwards so the schema
and SDKs stay in sync.

## End-to-end tests

Opt-in integration tests now live under [tests/e2e](e2e). Shared helpers in
[tests/e2e/internal/testenv](e2e/internal/testenv/env.go) centralize the environment variables
each suite needs. Every test file is guarded by a build tag so you only run the flows you have
credentials for:

- Subject verification: `-tags "e2e subjectverification"`
- Unified consent reads: `-tags "e2e consentread"`
- Consent writes: `-tags "e2e consentwrite"`

These suites call Osano's Unified Consent and subject-verification APIs directly through
`tests/e2e/internal/api`; they do not run the provider binary. They confirm the upstream contract the
provider relies on. The full Pulumi workflow (configuration, rules, publication, script outputs) is
covered by mocked provider tests and by the opt-in example deployment described in the
[end-to-end workflow guide](../docs/end-to-end-workflow.md).

In addition to the build tag, each suite checks for an opt-in flag before hitting the live API:

| Suite | Opt-in env | Description |
| --- | --- | --- |
| Subject verification | `OSANO_RUN_SUBJECT_E2E` | Drives send/verify subject code invokes |
| Consent reads | `OSANO_RUN_CONSENT_E2E` | Covers unified consent, subjects, profiles, config, and collections |
| Consent writes | `OSANO_RUN_WRITE_E2E` | Submits a real consent record and validates read-back |

The GitHub Actions acceptance workflow runs the read-only suite automatically when the required
repository secrets are configured. Write and subject-verification suites remain opt-in/manual because
they either mutate live data or require a one-time verification code.

CI compiles every build-tag set on each pull request, even without secrets, so a broken e2e suite
fails fast. Run the same check locally with:

```bash
make test_e2e_compile
```

The `acceptance_reads` CI job reads these repository secrets and skips itself (reporting success)
when any required one is missing:

| Secret | Required | Purpose |
| --- | --- | --- |
| `OSANO_UC_API_KEY` | Yes | Unified Consent API key |
| `OSANO_TEST_SUBJECT_REF` | Yes | Subject used for lookups |
| `OSANO_TEST_CONFIG_ID` | Yes | Config ID for consent-profile reads |
| `OSANO_TEST_HASHED_SUBJECT_ID` | Yes | Hashed subject for consent-profile reads |
| `OSANO_TEST_COLLECTION_ID` | Yes | Collection for the direct collection lookup |
| `OSANO_TEST_REFERENCE_TYPE` | No | `subject` (default) or `anonymous` |
| `OSANO_TEST_COLLECTIONS_JURISDICTION`, `OSANO_TEST_COLLECTIONS_TYPE` | No | Collection filters |
| `OSANO_API_BASE_URL` | No | Non-default Unified Consent host |

All suites honor an optional `OSANO_API_BASE_URL` to target a non-default Unified Consent host.
Cookie Consent resources have no live e2e suite; their lifecycle is covered by mocked HTTP tests in
`provider/cookie_consent_*_test.go`, and Unified Consent invokes by `provider/unified_consent_test.go`.

### Subject verification flow

Test location: [tests/e2e/subject_verification_test.go](e2e/subject_verification_test.go)

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

Test location: [tests/e2e/unified_consent_read_test.go](e2e/unified_consent_read_test.go)

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

Test location: [tests/e2e/consent_write_test.go](e2e/consent_write_test.go)

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
