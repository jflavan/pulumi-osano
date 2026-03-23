# Examples

The examples directory contains runnable Pulumi programs that show how to interact with the Osano provider.

| Example | Description |
| --- | --- |
| [quickstart](./quickstart) | Minimal consent submission in TypeScript, Python, and Go |

Each example expects the following config values:

- `osano:unifiedConsentApiKey` – `pulumi config set osano:unifiedConsentApiKey --secret`
- `subjectRef` – subject reference to operate on
- `subjectType` – optional, either `verified` (default) or `anonymous`
- `privacyProtocolId` / `configId` – values from the Osano dashboard

These examples are intended for use from a repository clone. The TypeScript example depends on the locally built Node.js SDK artifact, so run `mise exec -- make build_sdks` once before `npm install`.
