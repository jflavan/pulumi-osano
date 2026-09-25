# Security Analysis

The provider calls two Osano APIs on behalf of your Pulumi program: the Customer REST API (Cookie Consent configurations, rules, publication, discoveries, and audit log) and the Unified Consent API (consent submissions and lookups). Treat it as part of your compliance boundary.

## Credentials

- Two independent headers exist:
  - `x-osano-api-key` (`osano:osanoApiKey` / `OSANO_API_KEY`): Customer REST API Cookie Consent operations and functions, plus the subject send-code/verify routes.
  - `x-uc-api-key` (`osano:unifiedConsentApiKey` / `OSANO_UC_API_KEY`): Unified Consent submission and lookup routes. The subject send-code/verify routes send every configured key, because Osano's guide and its OpenAPI spec name different keys for them.
- Store both with `pulumi config set osano:... --secret`. CI/CD can inject them through `OSANO_API_KEY` and `OSANO_UC_API_KEY`; environment variables take precedence over stack config.
- Rotate keys regularly. The provider reads configuration fresh on every Pulumi operation, so the next `pulumi preview`, `up`, or `refresh` uses the rotated value.

## Network Access

- Cookie Consent requests go to `https://api.osano.com` unless you override `osano:customerBaseUrl`.
- Unified Consent requests go to `https://uc.api.osano.com` unless you override `osano:apiBaseUrl` or set `OSANO_API_BASE_URL`. Osano serves the Unified Consent API only from that host and routes regional processing internally (`us-east-1` for the US, `eu-central-1` for everything else).
- The provider honors standard proxy variables (`HTTPS_PROXY`, `NO_PROXY`) because it uses the default Go HTTP transport.

## Data in Transit and at Rest

- Consent payloads contain subject identifiers and potentially IP addresses or tags. They exist in memory inside the provider and in flight to Osano. Without `countryCodeOverride` and `regionCodeOverride`, Osano geolocates the caller's IP address, which in a pipeline is the CI runner's.
- Pulumi state stores the arguments you provide. Do not put subject secrets into plain-text config or resource inputs.
- The schema marks these values secret: `verifySubjectCode.code`, the SMS `session` of `sendSubjectCode` and `verifySubjectCode`, `Consent.sessionToken`, `getSession.sessionId`, the `destination` output of both verification functions, `verifySubjectCode.profile`, and the personal-data outputs of `getSubjectProfile` (`email`, `profile`) and `getSession` (`profile`). The email addresses and phone numbers passed to the verification functions are plain inputs, so prefer calling those functions from automation rather than long-lived stacks.
- `getCookieConsentAuditLog` returns the email address of the Osano user behind each event (`actor`), which is not marked secret.
- `CookieConsentPublication.webhookUrl` is marked secret. Osano calls it without authentication and does not document or sign the payload, so use an unguessable URL and treat the call only as a signal to check the configuration, for example with `getCookieConsentAuditLog`.
- `CookieConsentPublication.scriptSrc` and `scriptTag`, and the same outputs of `getCookieConsentConfig` and `getCookieConsentConfigs`, are public values intended for your site's HTML and are not secret.

## Website Integration

- Osano's script changes with every publication, every Osano CMP release, and by visitor location, so Subresource Integrity and hash-based Content Security Policy sources cannot pin it. Nonces work. The [end-to-end workflow guide](end-to-end-workflow.md#8-content-security-policy) lists the Content Security Policy sources Osano needs.
- Every environment where `osano.js` loads counts toward Osano traffic; give each environment its own configuration.

## Auditing

- Use Pulumi's audit logs (stacks and deployments) to see when consent submissions and Cookie Consent publications were triggered.
- `getCookieConsentAuditLog` reads Osano's Cookie Consent audit log: publications and configuration and rule changes, with the actor and time, including edits made in the Osano dashboard.
- Osano keeps its own immutable consent log; cross-reference Pulumi deployment IDs with the `attributes` you include in consent requests.

## Supply Chain

- The SDKs set `pluginDownloadURL` to `github://api.github.com/jflavan/pulumi-osano`, so Pulumi downloads the provider plugin from this repository's GitHub releases.
- Each release archive and its SBOM carry a GitHub build provenance attestation. Verify an archive before you trust it with `gh attestation verify pulumi-resource-osano-vX.Y.Z-linux-amd64.tar.gz --owner jflavan`.
- [PUBLISHING.md](PUBLISHING.md) lists every package, how it is published, and how to verify its provenance or signature.

## Future Work

- Add support for customer-managed encryption headers when Osano exposes them.
- Expose a read-only provider configuration that restricts mutation routes for read-heavy workloads.

For vulnerability disclosures see [SECURITY.md](../SECURITY.md).
