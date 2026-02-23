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
- 39 tests passing (37 unit + 2 integration skeletons) across provider + internal/osano packages
- Schema generated, SDKs generated for nodejs/python/go/dotnet
- Two TypeScript examples in `examples/`
- URL path parameters escaped with `url.PathEscape` to prevent path traversal
- CI via GitHub Actions (build, test, vet, artifact upload on push/PR to `main`)
- MIT license and CONTRIBUTING.md added

---

## Task 1: Add retry/backoff to HTTP client ✅
**File:** `provider/internal/osano/client.go`
- Add retry with exponential backoff for 429 and 5xx responses
- Respect `Retry-After` header if present
- Max 3 retries, configurable

## Task 2: Unit tests for CookieConsentConfig ✅
**Files:** `provider/cookie_consent_config_test.go`
- Test Check: valid inputs pass, missing name/domains/mode/storagePolicyHref fail
- Test Diff: same inputs → no changes, changed name/mode → update, changed config key → update
- Test bytesEqual helper

## Task 3: Add CookieConsentRule resource ✅
**Spec:** Customer REST API `/v1/cookie-consent/configs/{configId}/rules`
- Supports POST (create), PATCH (update), DELETE
- Fields: classification, rule, disclosure, title, vendorName
- This one has actual delete support
- Wire into `provider/provider.go` WithResources

## Task 4: Add Makefile ✅
**File:** `Makefile` (repo root)
- Targets: `provider` (build binary to `bin/`), `schema` (get-schema), `sdk/%` (gen-sdk per language), `clean`, `test`

## Task 5: Schema generation + SDK scaffolding ✅
- Schema generated via `make schema`
- SDKs generated for nodejs/python/go/dotnet via `make sdk/<lang>`

## Task 6: Add a minimal example ✅
**Dir:** `examples/cookie-consent-config-ts/` — Config + rule (TypeScript)
**Dir:** `examples/text-customization-ts/` — Text customization (TypeScript)

## Task 7: GitHub Actions CI ✅
**Dir:** `.github/workflows/ci.yml`
- Build provider, run tests, go vet, upload artifact on push/PR to `main`

---

## Notes
- Osano Customer REST API auth: `x-osano-api-key` header
- Osano UC Core API auth: `x-uc-api-key` header
- No delete endpoint exists for CMP configs (delete is no-op)
- CMP rules DO have delete
- Provider name: `osano`
- Go module: `github.com/jflavan/pulumi-osano/provider`
