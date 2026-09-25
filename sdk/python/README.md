# Osano Pulumi Provider

[![Build Status](https://img.shields.io/github/actions/workflow/status/jflavan/pulumi-osano/build.yml?branch=main)](https://github.com/jflavan/pulumi-osano/actions/workflows/build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/jflavan/pulumi-osano/blob/main/LICENSE)
[![npm version](https://img.shields.io/npm/v/@jflavan/pulumi-osano?label=npm&logo=npm)](https://www.npmjs.com/package/@jflavan/pulumi-osano)
[![PyPI version](https://img.shields.io/pypi/v/pulumi-osano?label=PyPI&logo=pypi&logoColor=white)](https://pypi.org/project/pulumi-osano/)
[![NuGet version](https://img.shields.io/nuget/v/Community.Pulumi.Osano?label=NuGet&logo=nuget)](https://www.nuget.org/packages/Community.Pulumi.Osano)
[![Go module version](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fproxy.golang.org%2Fgithub.com%2Fjflavan%2Fpulumi-osano%2Fsdk%2Fgo%2Fosano%2F%40latest&query=%24.Version&label=Go&logo=go&logoColor=white)](https://pkg.go.dev/github.com/jflavan/pulumi-osano/sdk/go/osano)
[![Maven Central version](https://img.shields.io/maven-central/v/io.github.jflavan.pulumi/pulumi-osano?label=Maven%20Central&logo=apachemaven)](https://central.sonatype.com/artifact/io.github.jflavan.pulumi/pulumi-osano)

> **⚠️ Unofficial community provider**
>
> This repo is not affiliated with Osano or Pulumi. It is maintained by the community and provided "as is" under the MIT license. Use GitHub Issues/Discussions for support.

The provider lets you manage Osano Cookie Consent and Unified Consent workflows alongside the rest of your infrastructure-as-code. You can:

- Create Cookie Consent configurations and rules, publish them after all dependencies settle, and export the hosted CMP script URL and exact HTML tag.
- Submit consent decisions programmatically from Pulumi deployments.
- Query unified consent state for a subject using Pulumi invokes.
- Resolve anonymous vs. verified subject identifiers via the `osano.getSubject` invoke when stitching identity flows.
- Inspect UC configuration and privacy protocol collections with `osano.getConfig`, `osano.getCollections`, and `osano.getCollection` invokes.
- Check for existing consent state and hashed consent profiles with `osano.checkConsent` and `osano.getConsentProfile`.
- Start and verify subject-profile challenges via `osano.sendSubjectCode` and `osano.verifySubjectCode` (requires the Osano API key). Pulumi runs invokes on every preview, update, and refresh, so call these two from automation rather than declaring them in a long-lived stack; otherwise each run sends a new code.
- Wire Osano calls into your CI/CD pipelines with first-class Node.js, Python, Go, .NET, and Java SDKs.

## Table of contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick start](#quick-start)
4. [Cookie Consent end to end](#cookie-consent-end-to-end)
5. [Authentication](#authentication)
6. [Configuration](#configuration)
7. [Examples](#examples)
8. [Development](#development)

---

## Prerequisites

- Pulumi CLI v3+
- API access to an Osano tenant (a Customer REST API key for Cookie Consent, a Unified Consent API key for Unified Consent, or both for mixed workloads)
- Runtime for your preferred language (Node.js 18+, Python 3.9+, Go 1.24+, .NET 8, or Java 11)

## Installation

The SDK declares its provider plugin, and Pulumi downloads the matching release from GitHub the first time you run `pulumi preview` or `pulumi up`. To install it manually, pin the version and point Pulumi at the GitHub releases:

```bash
pulumi plugin install resource osano <version> --server github://api.github.com/jflavan/pulumi-osano
```

To add the provider to a Pulumi program, reference the matching SDK:

- **Node.js**: `npm install @jflavan/pulumi-osano`
- **Python**: `pip install pulumi-osano`
- **Go**: `go get github.com/jflavan/pulumi-osano/sdk/go/osano`
- **.NET**: `dotnet add package Community.Pulumi.Osano`
- **Java**: `implementation("io.github.jflavan.pulumi:pulumi-osano:<version>")`

## Quick start

The TypeScript snippet below assumes you created a standard Pulumi TypeScript project with `pulumi new typescript` and then installed the released SDK:

```bash
npm install @jflavan/pulumi-osano
pulumi config set osano:unifiedConsentApiKey --secret
pulumi config set subjectRef <subject-id> --secret
pulumi config set configId <config-id>
pulumi config set privacyProtocolId <protocol-id>
pulumi up
```

`index.ts`:

```ts
import * as pulumi from "@pulumi/pulumi";
import * as osano from "@jflavan/pulumi-osano";

const cfg = new pulumi.Config();
const subjectRef = cfg.requireSecret("subjectRef");
const configId = cfg.require("configId");
const privacyProtocolId = cfg.require("privacyProtocolId");
const subjectType = cfg.get("subjectType") ?? "verified";

const subject = subjectRef.apply((value) =>
  subjectType === "anonymous" ? { anonymousId: value } : { verifiedId: value }
);

const consent = new osano.Consent("example", {
  subject,
  actions: [
    { target: privacyProtocolId, vendor: configId, action: "ACCEPT" },
  ],
  attributes: { pulumiStack: pulumi.getStack() },
  origin: "api",
  tags: ["demo"],
});

export const consentId = consent.consentId;
```

Run `pulumi up` to submit the consent. Destroying the stack removes the logical Pulumi resource but does **not** delete historical events from Osano (they are immutable).

If you're working from a repository clone instead of published packages, the repo-local examples under [examples/quickstart](https://github.com/jflavan/pulumi-osano/tree/main/examples/quickstart) are aimed at contributors. Run `mise exec -- make nodejs_sdk` once before using the TypeScript example so the local Node.js package exists.

## Cookie Consent end to end

The canonical [C# Cookie Consent example](https://github.com/jflavan/pulumi-osano/tree/main/examples/cookie-consent) creates a CMP configuration and its rules, then uses `CookieConsentPublication` to publish only after those resources settle. The companion TypeScript example implements the same lifecycle. Both compute a deterministic `changeToken`, declare explicit [`dependsOn`](https://www.pulumi.com/docs/iac/concepts/resources/options/dependson/) relationships, and allow a twenty-minute [`customTimeouts`](https://www.pulumi.com/docs/iac/concepts/resources/options/customtimeouts/) window.

Cookie Consent resources require a Customer REST API key:

```bash
export OSANO_API_KEY="replace-with-a-customer-rest-api-key"
```

After publication succeeds, the resource exposes these exact public outputs:

```csharp
var publication = new CookieConsentPublication(/* ... */);

return new Dictionary<string, object?>
{
    ["cookieConsentScriptSrc"] = publication.ScriptSrc, // scriptSrc
    ["cookieConsentScriptTag"] = publication.ScriptTag, // scriptTag
};
```

`scriptSrc` has the form `https://cmp.osano.com/{customerId}/{configId}/osano.js`; `scriptTag` is exactly `<script src="{scriptSrc}"></script>`. These installation values are deliberately non-secret. Put the returned tag first in the site `<head>` without `async` or `defer`, so the CMP loads before scripts it may control. Publication completion and CDN propagation are separate; the latest revision may take up to 15 minutes to reach every edge location.

See Osano's [Consent JavaScript API](https://developers.osano.com/cmp/javascript-api/developer-documentation-consent-javascript-api) and direct Customer REST API [`publishConfig` operation](https://developers.osano.com/customer-rest-api#tag/cmp/operation/publishConfig) for the upstream contracts.

## Authentication

Two API keys exist:

| Key | Header | Usage |
| --- | --- | --- |
| Unified Consent API key | `x-uc-api-key` | Required for consent submissions and read operations |
| Osano Customer REST API key | `x-osano-api-key` | Required for Cookie Consent configuration, rule, and publication resources; also used by the `sendSubjectCode` and `verifySubjectCode` functions |

Configure them with Pulumi config:

```bash
pulumi config set osano:unifiedConsentApiKey --secret
pulumi config set osano:osanoApiKey --secret   # required for Cookie Consent and subject verification
```

Or set environment variables for CI:

```bash
export OSANO_UC_API_KEY="..."
export OSANO_API_KEY="..."
```

## Configuration

Provider-level settings (all optional unless noted):

| Key | Description |
| --- | --- |
| `osano:unifiedConsentApiKey` | Unified Consent API key (secret); `OSANO_UC_API_KEY` takes precedence when set |
| `osano:osanoApiKey` | Customer REST API key for Cookie Consent and subject verification (secret); `OSANO_API_KEY` takes precedence when set |
| `osano:apiBaseUrl` | Override the Unified Consent API base URL, including any path prefix; defaults to `https://uc.api.osano.com`; `OSANO_API_BASE_URL` takes precedence when set |
| `osano:customerBaseUrl` | Override the Customer REST API base URL; defaults to `https://api.osano.com` |
| `osano:requestTimeoutSeconds` | HTTP timeout for Customer REST and Unified Consent calls, default 60 seconds; `OSANO_API_TIMEOUT_SECONDS` takes precedence when valid |

The deprecated `osano:ucApiKey` and `osano:ucBaseUrl` keys are still read as fallbacks for `unifiedConsentApiKey` and `apiBaseUrl`.

Resource-level inputs are documented in the generated SDKs, for example the [Go package reference](https://pkg.go.dev/github.com/jflavan/pulumi-osano/sdk/go/osano).

## Examples

- [examples/quickstart/typescript](https://github.com/jflavan/pulumi-osano/tree/main/examples/quickstart/typescript)
- [examples/quickstart/python](https://github.com/jflavan/pulumi-osano/tree/main/examples/quickstart/python)
- [examples/quickstart/go](https://github.com/jflavan/pulumi-osano/tree/main/examples/quickstart/go)
- [examples/cookie-consent](https://github.com/jflavan/pulumi-osano/tree/main/examples/cookie-consent) (canonical C# and companion TypeScript)

These repo-local examples contain `Pulumi.yaml` plus language-specific dependency files. The shared quickstart README documents the local SDK setup required when running them from a clone.

## Development

1. Install toolchain dependencies: `eval "$(mise activate zsh)" && mise install`
2. Build the provider: `make provider`
3. Run tests: `make test_provider`
4. Regenerate schema + SDKs after editing Go code: `make codegen`

For the full lifecycle (install, deploy, day-2 changes, import, teardown, and the contributor loop) see the [end-to-end workflow guide](https://github.com/jflavan/pulumi-osano/blob/main/docs/end-to-end-workflow.md). See [CONTRIBUTING.md](https://github.com/jflavan/pulumi-osano/blob/main/CONTRIBUTING.md) and the [docs](https://github.com/jflavan/pulumi-osano/tree/main/docs) for release instructions, troubleshooting tips, and workflows.
