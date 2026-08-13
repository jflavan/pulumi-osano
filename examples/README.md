# Examples

The examples directory contains runnable Pulumi programs that show how to interact with the Osano provider.

| Example | Description |
| --- | --- |
| [quickstart](./quickstart) | Minimal consent submission in TypeScript, Python, and Go |
| [cookie-consent](./cookie-consent) | End-to-end Cookie Consent configuration, rules, publication, and hosted script outputs in canonical C# and companion TypeScript |

## Quickstart credentials and configuration

The quickstart uses Unified Consent:

- `osano:unifiedConsentApiKey` – `pulumi config set osano:unifiedConsentApiKey --secret`
- `subjectRef` – subject reference to operate on
- `subjectType` – optional, either `verified` (default) or `anonymous`
- `privacyProtocolId` / `configId` – values from the Osano dashboard

## Cookie Consent credentials and configuration

The Cookie Consent workflow requires a Customer REST API key through
`OSANO_API_KEY` or `pulumi config set osano:osanoApiKey --secret`, plus the
example's `domain`, `storagePolicyHref`, and optional `mode` stack values. See
its [README](./cookie-consent) before opting in to `pulumi up`: it creates and
publishes real customer resources.

These examples are intended for use from a repository clone. The quickstart
TypeScript example depends on the locally built Node.js SDK artifact, so run
`mise exec -- make build_sdks` once before `npm install`. For Cookie Consent,
run `mise exec -- make build_cookie_consent_examples`; it materializes the
local SDK artifacts and compiles both examples without contacting Osano.
