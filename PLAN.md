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
- `osano:apiToken` (secret)
- `osano:baseUrl` (optional; default Osano API URL)
- Optional: `customerId` / `configId` defaults if Osano’s API is scoped that way

### First resources (pick 1–2 to ship quickly)
Because the UC Core API appears to have a large **Config** object with many nested fields, start with a **coarse-grained** resource:

1) `osano:uc:CoreConfig` (or `osano:uc:Config`)
- Inputs: fields you want to manage declaratively (domains, privacyPolicyUrl, processingTime/unit, privacyProtocols, text customizations, styling, etc.)
- Outputs: server-managed fields (created/updated/published IDs, etc.)
- Behavior: create/update calls map to the API endpoints for config update; diff ignores server-managed fields.

Then iterate into fine-grained resources once patterns are solid:
- `Domain` (if separate endpoint exists)
- `PrivacyProtocol` / `Integration` (if CRUD-able independently)
- `TextCustomization` (per locale)

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
- [ ] Implement `client` package:
  - auth header handling
  - base URL
  - request/response structs (generated from OpenAPI or hand-written)
  - retry/backoff for 429/5xx
  - pagination helpers if needed

### Phase 2 — Provider + CoreConfig resource
- [ ] Provider `Configure` parses config, validates token.
- [ ] Implement `CoreConfig` resource with:
  - `Check` validation
  - `Diff` ignoring server-managed fields
  - `Create/Read/Update/Delete` mapping to Osano endpoints
- [ ] Add import support (document `pulumi import` usage).

### Phase 3 — Testing
- [ ] Unit tests for diff/normalize logic.
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
