# Security Analysis

The provider calls two Osano APIs on behalf of your Pulumi program: the Customer REST API (Cookie Consent configurations, rules, and publication) and the Unified Consent API (consent submissions and lookups). Treat it as part of your compliance boundary.

## Credentials

- Two independent headers exist:
  - `x-osano-api-key` (`osano:osanoApiKey` / `OSANO_API_KEY`): Customer REST API Cookie Consent operations, plus the subject send-code/verify routes.
  - `x-uc-api-key` (`osano:unifiedConsentApiKey` / `OSANO_UC_API_KEY`): Unified Consent submission and lookup routes.
- Store both with `pulumi config set osano:... --secret`. CI/CD can inject them through `OSANO_API_KEY` and `OSANO_UC_API_KEY`; environment variables take precedence over stack config.
- Rotate keys regularly. The provider reads configuration fresh on every Pulumi operation, so the next `pulumi preview`, `up`, or `refresh` uses the rotated value.

## Network Access

- Cookie Consent requests go to `https://api.osano.com` unless you override `osano:customerBaseUrl`.
- Unified Consent requests go to `https://uc.api.osano.com` unless you override `osano:apiBaseUrl` or set `OSANO_API_BASE_URL`.
- The provider honors standard proxy variables (`HTTPS_PROXY`, `NO_PROXY`) because it uses the default Go HTTP transport.

## Data in Transit and at Rest

- Consent payloads contain subject identifiers and potentially IP addresses or tags. They exist in memory inside the provider and in flight to Osano.
- Pulumi state stores the arguments you provide. Do not put subject secrets into plain-text config or resource inputs.
- `verifySubjectCode.code` is marked secret in the schema. Email addresses and phone numbers passed to the verification functions are not, so prefer calling those functions from automation rather than long-lived stacks.
- `CookieConsentPublication.scriptSrc` and `scriptTag` are public values intended for your site's HTML and are not secret.

## Auditing

- Use Pulumi's audit logs (stacks and deployments) to see when consent submissions and Cookie Consent publications were triggered.
- Osano keeps its own immutable log; cross-reference Pulumi deployment IDs with the `origin` or `attributes` fields you include in consent requests.

## Future Work

- Add support for customer-managed encryption headers when Osano exposes them.
- Expose a read-only provider configuration that restricts mutation routes for read-heavy workloads.

For vulnerability disclosures see `SECURITY.md`.
