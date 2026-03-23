# Osano Pulumi Provider

[![Build Status](https://img.shields.io/github/actions/workflow/status/jflavan/pulumi-osano/build.yml?branch=main)](https://github.com/jflavan/pulumi-osano/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![npm version](https://img.shields.io/npm/v/@jflavan/pulumi-osano)](https://www.npmjs.com/package/@jflavan/pulumi-osano)
[![PyPI version](https://img.shields.io/pypi/v/pulumi-osano)](https://pypi.org/project/pulumi-osano/)
[![NuGet version](https://img.shields.io/nuget/v/Community.Pulumi.Osano)](https://www.nuget.org/packages/Community.Pulumi.Osano)
[![Go Reference](https://pkg.go.dev/badge/github.com/jflavan/pulumi-osano/sdk/go/osano.svg)](https://pkg.go.dev/github.com/jflavan/pulumi-osano/sdk/go/osano)

> **⚠️ Unofficial community provider**
>
> This repo is not affiliated with Osano or Pulumi. It is maintained by the community and provided "as is" under the MIT license. Use GitHub Issues/Discussions for support.

The provider lets you manage Osano Unified Consent workflows alongside the rest of your infrastructure-as-code. You can:

- Submit consent decisions programmatically from Pulumi deployments.
- Query unified consent state for a subject using Pulumi invokes.
- Resolve anonymous vs. verified subject identifiers via the `osano.getSubject` invoke when stitching identity flows.
- Inspect UC configuration and privacy protocol collections with `osano.getConfig`, `osano.getCollections`, and `osano.getCollection` invokes.
- Check for existing consent state and hashed consent profiles with `osano.checkConsent` and `osano.getConsentProfile`.
- Start and verify subject-profile challenges via `osano.sendSubjectCode` and `osano.verifySubjectCode` (requires the Osano API key).
- Wire Osano calls into your CI/CD pipelines with first-class Node.js, Python, Go, .NET, and Java SDKs.

## Table of contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick start](#quick-start)
4. [Authentication](#authentication)
5. [Configuration](#configuration)
6. [Examples](#examples)
7. [Development](#development)

---

## Prerequisites

- Pulumi CLI v3+
- API access to an Osano tenant (Unified Consent API key, optionally an Osano API key)
- Runtime for your preferred language (Node.js 18+, Python 3.9+, Go 1.24+, .NET 8, or Java 11)

## Installation

The plugin is installed automatically the first time you run `pulumi up`. To install manually:

```bash
pulumi plugin install resource osano
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

If you're working from a repository clone instead of published packages, the repo-local examples under [examples/quickstart](./examples/quickstart) are aimed at contributors. Run `mise exec -- make build_sdks` once before using the TypeScript example so the local Node.js package exists.

## Authentication

Two API keys exist:

| Key | Header | Usage |
| --- | --- | --- |
| Unified Consent API key | `x-uc-api-key` | Required for consent submissions and read operations |
| Osano API key | `x-osano-api-key` | Needed for administrative endpoints (subjects, merges, verification) |

Configure them with Pulumi config:

```bash
pulumi config set osano:unifiedConsentApiKey --secret
pulumi config set osano:osanoApiKey --secret   # optional today
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
| `osano:unifiedConsentApiKey` | Unified Consent API key (secret) |
| `osano:osanoApiKey` | Osano API key (secret) |
| `osano:apiBaseUrl` | Override the API base URL; defaults to `https://uc.api.osano.com` |
| `osano:requestTimeoutSeconds` | HTTP timeout, default 60 seconds |

Resource-level inputs are documented in the auto-generated SDK docs (see the GoDoc badge above).

## Examples

- [examples/quickstart/typescript](./examples/quickstart/typescript)
- [examples/quickstart/python](./examples/quickstart/python)
- [examples/quickstart/go](./examples/quickstart/go)

These repo-local examples contain `Pulumi.yaml` plus language-specific dependency files. The shared quickstart README documents the local SDK setup required when running them from a clone.

## Development

1. Install toolchain dependencies: `eval "$(mise activate zsh)" && mise install`
2. Build the provider: `make provider`
3. Run tests: `make test_provider`
4. Regenerate schema + SDKs after editing Go code: `make codegen`

See [CONTRIBUTING.md](./CONTRIBUTING.md) and the docs under `./docs` for release instructions, troubleshooting tips, and workflows.
