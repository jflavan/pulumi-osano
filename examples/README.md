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
- `jurisdiction` – optional jurisdiction applied to the consent action

## Cookie Consent credentials and configuration

The Cookie Consent workflow requires a Customer REST API key through
`OSANO_API_KEY` or `pulumi config set osano:osanoApiKey --secret`, plus the
example's `domain`, `storagePolicyHref`, and optional `mode` stack values. See
its [README](./cookie-consent) before opting in to `pulumi up`: it creates and
publishes real customer resources.

These examples are intended for use from a repository clone: their dependency
files reference the SDKs generated in this repository, and running them needs a
locally built provider plugin (see the [quickstart README](./quickstart/README.md)
for the plugin install commands). The quickstart TypeScript example depends on
the locally built Node.js SDK artifact, so run `mise exec -- make nodejs_sdk`
once before `npm install`. For Cookie Consent, run
`mise exec -- make build_cookie_consent_examples`; it materializes the local SDK
artifacts and compiles both examples without contacting Osano.
`mise exec -- make build_examples` compiles the Cookie Consent examples and the
Go quickstart together.

To run an example against the published packages instead, see
[Run against the published packages](./quickstart/README.md#run-against-the-published-packages)
and, for Cookie Consent, the released-package sections of its
[README](./cookie-consent/README.md). [docs/PUBLISHING.md](../docs/PUBLISHING.md)
lists every published package.
