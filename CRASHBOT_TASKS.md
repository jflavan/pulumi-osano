# Crashbot Tasks — pulumi-osano

Project: `~/projects/pulumi-osano`
Provider binary: `provider/cmd/pulumi-resource-osano`
Reference repo: `.ref/pulumi-webflow` (cloned locally)

## Current state
- Provider scaffolded in Go using `pulumi-go-provider` v1.3.0
- Builds successfully: `make provider` (or `cd provider && go build ./cmd/pulumi-resource-osano`)
- Two resources implemented: `CookieConsentConfig` and `CookieConsentRule`
- HTTP client in `provider/internal/osano/` with retry/backoff, Retry-After support
- OpenAPI specs in `openapi/` for reference
- Unit tests passing (37 tests across provider + internal/osano packages)
- Schema generated, SDKs generated for nodejs/python/go/dotnet
- Minimal TypeScript example in `examples/cookie-consent-config-ts/`
- URL path parameters escaped with `url.PathEscape` to prevent path traversal
- No CI yet

---

## Task 1: Add retry/backoff to HTTP client
**File:** `provider/internal/osano/client.go`
- Add retry with exponential backoff for 429 and 5xx responses
- Respect `Retry-After` header if present
- Max 3 retries, configurable

## Task 2: Unit tests for CookieConsentConfig
**Files:** `provider/cookie_consent_config_test.go`
- Test Check: valid inputs pass, missing name/domains/mode/storagePolicyHref fail
- Test Diff: same inputs → no changes, changed name/mode → update, changed config key → update
- Test bytesEqual helper

## Task 3: Add CookieConsentRule resource
**Spec:** Customer REST API `/v1/cookie-consent/configs/{configId}/rules`
- Supports POST (create), PATCH (update), DELETE
- Fields: classification, rule, disclosure, title, vendorName
- This one has actual delete support
- Wire into `provider/provider.go` WithResources

## Task 4: Add Makefile
**File:** `Makefile` (repo root)
- Reference: `.ref/pulumi-webflow/Makefile`
- Targets: `provider` (build binary to `bin/`), `schema` (get-schema), `sdk/%` (gen-sdk per language), `clean`
- Keep it minimal for now

## Task 5: Schema generation + SDK scaffolding
- After Makefile: `make provider && make schema`
- Then `make sdk/nodejs sdk/python sdk/go sdk/dotnet`
- Verify generated SDKs compile/lint

## Task 6: Add a minimal example
**Dir:** `examples/cookie-consent-config-ts/`
- TypeScript Pulumi program that creates a CookieConsentConfig
- Include `Pulumi.yaml` with provider plugin reference

## Task 7 (stretch): GitHub Actions CI
**Dir:** `.github/workflows/`
- Build provider on push
- Run tests
- Optional: generate schema + SDKs in CI

---

## Notes
- Osano Customer REST API auth: `x-osano-api-key` header
- Osano UC Core API auth: `x-uc-api-key` header
- No delete endpoint exists for CMP configs (delete is no-op)
- CMP rules DO have delete
- Provider name: `osano`
- Go module: `github.com/johnflavan/pulumi-osano/provider`
