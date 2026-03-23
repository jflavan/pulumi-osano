# Security Analysis

The provider communicates with the Osano Unified Consent API on behalf of your Pulumi program. Treat it as part of your compliance boundary.

## Credentials

- Two independent headers exist: `x-osano-api-key` (administrative routes) and `x-uc-api-key` (consent submission routes).
- Prefer storing both via `pulumi config set osano:... --secret`. CI/CD should inject them through environment variables (`OSANO_API_KEY`, `OSANO_UC_API_KEY`).
- Rotate keys regularly. When a key rotates, update the Pulumi config and re-run `pulumi refresh` to ensure the provider cache picks up the new value.

## Network Access

- All requests go to `https://uc.api.osano.com` unless you override `osano:apiBaseUrl`.
- The provider respects standard corporate proxy variables (`HTTPS_PROXY`) because it uses the default Go HTTP stack.

## Data in Transit

- Consent payloads contain subject identifiers and potentially IP addresses/tags. These values only exist in-memory inside the provider and in-flight to Osano.
- Pulumi state stores the arguments you provide. Do not put subject secrets or verification codes into plain-text config or resource inputs.

## Auditing

- Use Pulumi's audit logs (stacks + deployments) to see when consent submissions were triggered.
- Osano keeps its own immutable log; you can cross-reference Pulumi deployment IDs with the `origin` or `attributes` fields you include in requests.

## Future Work

- Add support for customer-managed encryption headers when Osano exposes them.
- Expose a read-only provider configuration that restricts mutation routes for read-heavy workloads.

For vulnerability disclosures see `SECURITY.md`.
