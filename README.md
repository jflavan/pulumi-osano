# pulumi-osano

Pulumi provider for the Osano UC Core API.

Status: scaffolding/plan stage.

## Docs
- Pulumi provider guide: https://www.pulumi.com/docs/iac/guides/building-extending/providers/build-a-provider/
- Osano UC Core OpenAPI: https://developers.osano.com/uc/core-api/openapi

## Plan
See [PLAN.md](./PLAN.md).

## Prereqs (for local dev)
- Go
- Pulumi CLI

## Local build
From repo root:

- Build provider binary:
  - `cd provider && go build ./cmd/pulumi-resource-osano`

## Notes
- Osano Customer REST API uses `x-osano-api-key`.
- Osano Unified Consent Core API uses `x-uc-api-key`.
- See `PLAN.md` for current scope and next steps.
