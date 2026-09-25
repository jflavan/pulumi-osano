# Quickstart

This example deploys the `osano:index:Consent` resource to submit a consent decision for a subject. Three language variants are provided:

- [TypeScript](./typescript)
- [Python](./python)
- [Go](./go)

## Prerequisites

1. Create or locate a Unified Consent API key in the Osano dashboard.
2. Identify the subject reference: a verified subject ID, or an anonymous ID when `subjectType` is `anonymous`.
3. Identify the privacy protocol target ID and configuration (vendor) ID that should receive the consent.
4. If you are running these examples from a repository clone, prepare the local SDK for your language and install the locally built provider plugin. The TypeScript and Python SDKs in the clone request the development plugin version `0.1.0-alpha.0+dev`, which is never published. The Go example resolves the SDK through a `replace` directive, so Pulumi requests the plugin version from its `go.mod` requirement, `0.0.0`; install the same binary under that version too:

   ```bash
   mise exec -- make nodejs_sdk   # TypeScript only; Python and Go use the checked-in SDK sources
   mise exec -- make provider
   mise exec -- pulumi plugin install resource osano 0.1.0-alpha.0+dev \
     --file ./bin/pulumi-resource-osano --exact --reinstall
   mise exec -- pulumi plugin install resource osano 0.0.0 \
     --file ./bin/pulumi-resource-osano --exact --reinstall   # Go only
   ```

## Running

From the language directory (`typescript`, `python`, or `go`), install the dependencies and create a stack:

```bash
npm install             # TypeScript only
pulumi install          # Python only: creates the venv and installs requirements.txt
pulumi stack init dev
```

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

Each language example loads the same config keys. Then run `pulumi up` to submit the consent.

In a clone, run these `pulumi` commands through `mise exec --` as well, or in a shell with mise activated. The repository's mise configuration sets `PULUMI_HOME` to `.pulumi` inside the clone, so a plain `pulumi` command uses `~/.pulumi` instead and does not find the plugin installed in step 4.

The examples in this folder are repo-local development examples. The Go example builds against the local SDK through a `replace` directive; `make build_quickstart_examples` compiles it.

## Run against the published packages

To run an example with the published packages instead of the clone's SDKs, copy its language directory out of the repository and switch the SDK reference:

- TypeScript: `npm install @jflavan/pulumi-osano@0.1.0` (replaces the `file:` dependency in `package.json`).
- Python: in `requirements.txt`, replace `-e ../../../sdk/python` with `pulumi-osano==0.1.0`.
- Go: `go mod edit -dropreplace=github.com/jflavan/pulumi-osano/sdk/go/osano -require=github.com/jflavan/pulumi-osano/sdk/go/osano@v0.1.0 && go mod tidy`

Skip the local plugin install in step 4 and the mise note above: the published SDKs request provider plugin 0.1.0, and Pulumi downloads it from GitHub on the first `pulumi preview` or `pulumi up`. [docs/PUBLISHING.md](../../docs/PUBLISHING.md) lists every published package.

Do not commit these changes to the repository: CI builds the examples against the local SDKs.
