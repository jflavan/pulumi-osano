---
title: Osano (Unofficial) Installation & Configuration
meta_desc: How to install the unofficial Osano provider for Pulumi and configure its Osano API keys, API endpoints, and request timeout.
layout: package
---

## Installation

The Osano (Unofficial) provider is available as a package in these Pulumi languages:

- JavaScript/TypeScript: [`@jflavan/pulumi-osano`](https://www.npmjs.com/package/@jflavan/pulumi-osano)

  ```bash
  npm install @jflavan/pulumi-osano
  ```

- Python: [`pulumi-osano`](https://pypi.org/project/pulumi-osano/)

  ```bash
  pip install pulumi-osano
  ```

- Go: [`github.com/jflavan/pulumi-osano/sdk/go/osano`](https://pkg.go.dev/github.com/jflavan/pulumi-osano/sdk/go/osano)

  ```bash
  go get github.com/jflavan/pulumi-osano/sdk/go/osano
  ```

- .NET: [`Community.Pulumi.Osano`](https://www.nuget.org/packages/Community.Pulumi.Osano)

  ```bash
  dotnet add package Community.Pulumi.Osano
  ```

- Java: [`io.github.jflavan.pulumi:pulumi-osano`](https://central.sonatype.com/artifact/io.github.jflavan.pulumi/pulumi-osano) on Maven Central. With Gradle, replacing `VERSION` with the release you want:

  ```groovy
  implementation 'io.github.jflavan.pulumi:pulumi-osano:VERSION'
  ```

### Provider plugin

The SDK packages record the plugin download location (`github://api.github.com/jflavan/pulumi-osano`), so Pulumi downloads the matching `pulumi-resource-osano` plugin from the provider's [GitHub releases](https://github.com/jflavan/pulumi-osano/releases) the first time you run `pulumi preview` or `pulumi up`. Pin the SDK package to an exact version and the engine installs the plugin of the same version.

To install the plugin by hand, for example on a machine that runs Pulumi YAML programs or before going offline, run:

```bash
pulumi plugin install resource osano VERSION --server github://api.github.com/jflavan/pulumi-osano
```

## Authentication

Osano exposes two APIs, and each uses its own key. Configure the key for the API you use, or both for programs that mix Cookie Consent and Unified Consent.

| Key | Header | Used by |
| --- | --- | --- |
| Customer REST API key (`osano:osanoApiKey`) | `x-osano-api-key` | `CookieConsentConfig`, `CookieConsentRule`, and `CookieConsentPublication`, and the `sendSubjectCode` and `verifySubjectCode` functions |
| Unified Consent API key (`osano:unifiedConsentApiKey`) | `x-uc-api-key` | The `Consent` resource and the `getUnifiedConsent`, `getSubject`, `getConfig`, `getCollections`, `getCollection`, `checkConsent`, and `getConsentProfile` functions |

Store keys as Pulumi secrets:

```bash
pulumi config set --secret osano:osanoApiKey <customer-rest-api-key>
pulumi config set --secret osano:unifiedConsentApiKey <unified-consent-api-key>
```

Or provide them through environment variables, for example in CI. An environment variable takes precedence over the stack configuration when it is set:

```bash
export OSANO_API_KEY="<customer-rest-api-key>"
export OSANO_UC_API_KEY="<unified-consent-api-key>"
```

A resource or function whose key is missing fails with an error naming the configuration key and environment variable to set.

## Configuration reference

All settings are optional at the provider level.

| Name | Environment variable | Secret | Default | Description |
| --- | --- | --- | --- | --- |
| `osanoApiKey` | `OSANO_API_KEY` | Yes | | Osano Customer REST API key for Cookie Consent resources and the `sendSubjectCode` and `verifySubjectCode` functions. |
| `unifiedConsentApiKey` | `OSANO_UC_API_KEY` | Yes | | Unified Consent API key for the `Consent` resource and the Unified Consent read functions. |
| `apiBaseUrl` | `OSANO_API_BASE_URL` | No | `https://uc.api.osano.com` | Base URL of the Unified Consent API, including any path prefix. Override it only for a custom domain. |
| `customerBaseUrl` | | No | `https://api.osano.com` | Base URL of the Customer REST API. |
| `requestTimeoutSeconds` | `OSANO_API_TIMEOUT_SECONDS` | No | `60` | HTTP request timeout, in seconds, for Customer REST API and Unified Consent calls. The environment variable is used only when it is a positive integer. |
| `ucApiKey` | | Yes | | Deprecated: use `unifiedConsentApiKey`. Read only when neither `unifiedConsentApiKey` nor `OSANO_UC_API_KEY` is set. |
| `ucBaseUrl` | | No | | Deprecated: use `apiBaseUrl`. Read only when neither `apiBaseUrl` nor `OSANO_API_BASE_URL` is set. |

Changing provider configuration, such as rotating an API key or adding `requestTimeoutSeconds`, updates the provider in place. It never replaces the Cookie Consent resources the provider manages.

## Configuration examples

### Cookie Consent only

```bash
pulumi config set --secret osano:osanoApiKey <customer-rest-api-key>
```

### Unified Consent only

```bash
pulumi config set --secret osano:unifiedConsentApiKey <unified-consent-api-key>
```

### Custom endpoints and a longer timeout

```bash
pulumi config set osano:apiBaseUrl https://consent.example.com/uc
pulumi config set osano:customerBaseUrl https://api.example.com
pulumi config set osano:requestTimeoutSeconds 120
```

### Environment variables only

```bash
export OSANO_API_KEY="<customer-rest-api-key>"
export OSANO_UC_API_KEY="<unified-consent-api-key>"
export OSANO_API_BASE_URL="https://uc.api.osano.com"
export OSANO_API_TIMEOUT_SECONDS=120
pulumi up
```
