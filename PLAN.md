# pulumi-osano — plan

## Goal
Create a Pulumi provider for the **Osano UC Core API** so Osano configuration can be managed in IaC pipelines (Pulumi up/preview) across multiple languages.

Docs:
- Pulumi: https://www.pulumi.com/docs/iac/guides/building-extending/providers/build-a-provider/
- Osano OpenAPI: https://developers.osano.com/uc/core-api/openapi
- Reference provider repo: https://github.com/JDetmar/pulumi-webflow/tree/main

## Recommended implementation approach
Use the **Pulumi Go Provider SDK** (`github.com/pulumi/pulumi-go-provider`) to implement a native provider:
- Provider runs as a gRPC server, Pulumi engine drives CRUD.
- SDK auto-generates schema; enables multi-language SDK generation.

> Note: this repo currently has no `go` or `pulumi` CLI available on the host machine. Either install prerequisites or develop on a machine/CI runner that has them.

## Scope (v0)
Start with a small, high-leverage surface area that supports idempotent pipeline automation.

### Provider configuration
Osano currently exposes multiple API surfaces with different auth headers:
- Customer REST API: `x-osano-api-key`
- Unified Consent Core API: `x-uc-api-key`

Provider config (proposed):
- `osano:osanoApiKey` (secret) — Customer REST API
- `osano:ucApiKey` (secret) — Unified Consent Core API
- `osano:customerBaseUrl` (optional; default `https://api.osano.com`)
- `osano:ucBaseUrl` (optional; default `https://uc.api.osano.com`)

### First resources (pick 1–2 to ship quickly)
**Customer REST API is the best v0 target for IaC** because it includes real CRUD for configuration objects (e.g., CMP configs/rules). The Unified Consent Core API is primarily runtime/consent flows and appears largely read-only for configuration.

1) `osano:CookieConsentConfig`
- Backed by `/v1/cookie-consent/configs` (create/list/get/patch)
- Inputs: name/domains/mode/orgIds + `configuration` object
- Outputs: `configId`, `publishStatus`, `lastPublished`, etc.
- Note: public spec does **not** include delete for configs (delete will be no-op / orphan).

2) (optional) `osano:CookieConsentConfigPublish`
- Backed by `/v1/cookie-consent/configs/{configId}/publish`
- Modeled carefully (publish is async + potentially non-idempotent); may be a Function/invoke instead.

Then iterate into fine-grained resources once patterns are solid:
- `CookieConsentRule` (`/rules` endpoints)
- `DataStore` connectors, etc.
- UC Core functions: `getConfig`, `getCollections`, token creation helpers

## Design decisions
- Prefer explicit IDs for import/refresh (Pulumi `Read`).
- Make operations idempotent (retry transient errors, tolerate “already exists”).
- Treat “unknown until apply” carefully; avoid forcing replacements on computed outputs.
- Document what the provider *doesn’t* manage yet.

## Repo structure (target)
- `provider/` — Go provider implementation
- `sdk/` — generated SDKs (nodejs/python/dotnet/go)
- `examples/` — minimal Pulumi programs
- `docs/` — usage docs

## Work plan / tasks
### Phase 0 — scaffolding
- [ ] Decide package name: `osano` (provider name) and module naming.
- [ ] Add license, README, contributing.
- [ ] Scaffold provider using pulumi-go-provider conventions:
  - `go.mod`
  - `PulumiPlugin.yaml`
  - `main.go` with provider wiring

### Phase 1 — Osano client
- [x] Implement `client` package:
  - auth header handling
  - base URL
  - request/response structs (generated from OpenAPI or hand-written)
  - [x] retry/backoff for 429/5xx (exponential backoff, Retry-After header support, configurable via `ClientOption`s)
  - pagination helpers if needed

### Phase 2 — Provider + CoreConfig resource
- [ ] Provider `Configure` parses config, validates token.
- [ ] Implement `CoreConfig` resource with:
  - `Check` validation
  - `Diff` ignoring server-managed fields
  - `Create/Read/Update/Delete` mapping to Osano endpoints
- [ ] Add import support (document `pulumi import` usage).

### Phase 3 — Testing
- [x] Unit tests for Check validation (valid inputs, missing name/domains/mode/storagePolicyHref).
- [x] Unit tests for Diff logic (same inputs → no changes, changed name/mode/configuration → update).
- [x] Unit test for `bytesEqual` helper.
- [ ] Optional integration tests (env vars for token, configId).

### Phase 4 — SDK gen + packaging
- [ ] Generate schema/SDKs for TS/Python/.NET/Go.
- [ ] Add CI (GitHub Actions) to build provider binaries and publish artifacts.

### Phase 5 — Examples
- [ ] Example: manage domains + privacyPolicyUrl.
- [ ] Example: manage text customization for en-US.

## Questions to answer early
- What’s the auth mechanism for UC Core API (Bearer token? API key header?)
- What are the stable identifiers (configId/customerId) and which endpoints support CRUD?
- Are there separate endpoints for sub-resources, or is everything patched through a single config endpoint?
- Any eventual consistency delays that require read-after-write polling?
