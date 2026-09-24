# Quickstart

This example deploys the `osano:index:Consent` resource to submit a consent decision for a subject. Three language variants are provided:

- [TypeScript](./typescript)
- [Python](./python)
- [Go](./go)

## Prerequisites

1. Create or locate a Unified Consent API key in the Osano dashboard.
2. Identify the subject reference: a verified subject ID, or an anonymous ID when `subjectType` is `anonymous`.
3. Identify the privacy protocol target ID and configuration (vendor) ID that should receive the consent.
4. If you are running these examples from a repository clone, build the local SDK artifacts first with `mise exec -- make build_sdks`.

Configure the stack before running any example:

```bash
pulumi config set osano:unifiedConsentApiKey --secret
pulumi config set subjectRef <subject-id> --secret
pulumi config set configId <config-id>
pulumi config set privacyProtocolId <protocol-id>
# Optional:
pulumi config set subjectType anonymous      # default: verified
pulumi config set jurisdiction <jurisdiction> # applied to the consent action
```

Each language example loads the same config keys.
The examples in this folder are repo-local development examples. For public package installation instructions, use the install commands in the repository [README](../../README.md). The Go example builds against the local SDK through a `replace` directive; `make build_quickstart_examples` compiles it.
