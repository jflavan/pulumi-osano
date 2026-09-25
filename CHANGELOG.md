# Changelog

All notable changes to the Osano (Unofficial) Pulumi provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Until 1.0.0, a minor release
(0.x.0) may contain breaking changes; see [docs/UPGRADE.md](docs/UPGRADE.md).

## [Unreleased]

### Added

- `docs/PUBLISHING.md` lists every published package (npm, PyPI, NuGet, Go, Maven Central) and the
  provider plugin, with install commands, how each one is published, and how to verify its
  provenance or signature.

## [0.1.0] - 2026-09-25

First public release. The provider is community maintained and is not affiliated with Osano, Inc.
or Pulumi Corporation.

### Added

- Cookie Consent (CMP) resources for the Osano Customer REST API:
  - `CookieConsentConfig` manages a CMP configuration: name, domains, compliance mode
    (`debug`, `permissive`, or `production`), the configuration object (which must include
    `storagePolicyHref`), and optional organization IDs. Import it with the Osano config ID.
  - `CookieConsentRule` manages a cookie, script, iframe, or localStorage classification rule with
    its rule type, description, expiry, disclosure, title, and vendor name. Import it with the
    composite ID `<configId>/<ruleId>`. Changing a rule's `configId` or `storeType` replaces it.
  - `CookieConsentPublication` publishes a configuration, waits for Osano to finish, and exports the
    public, non-secret `scriptSrc` (`https://cmp.osano.com/{customerId}/{configId}/osano.js`) and
    the exact `scriptTag` (`<script src="{scriptSrc}"></script>`) to place first in the site `<head>`.
- Unified Consent support for the Unified Consent API:
  - The `Consent` resource submits a consent decision for a verified or anonymous subject, with
    actions, attributes, compliance metadata, jurisdiction, origin, and tags.
  - Functions `getUnifiedConsent`, `getSubject`, `getConfig`, `getCollections`, `getCollection`,
    `checkConsent`, and `getConsentProfile` read consent, subject, configuration, and privacy
    protocol data, and `sendSubjectCode` and `verifySubjectCode` start and verify subject-profile
    challenges with the Customer REST API key.
- Provider configuration `osanoApiKey` (`OSANO_API_KEY`), `unifiedConsentApiKey`
  (`OSANO_UC_API_KEY`), `apiBaseUrl` (`OSANO_API_BASE_URL`), `customerBaseUrl`, and
  `requestTimeoutSeconds` (`OSANO_API_TIMEOUT_SECONDS`, default 60 seconds). A set environment
  variable takes precedence over stack configuration.
- SDKs for Node.js (`@jflavan/pulumi-osano`), Python (`pulumi-osano`), Go
  (`github.com/jflavan/pulumi-osano/sdk/go/osano`), .NET (`Community.Pulumi.Osano`), and Java
  (`io.github.jflavan.pulumi:pulumi-osano`). The SDKs download the matching provider plugin from
  GitHub releases (`github://api.github.com/jflavan/pulumi-osano`) on first use.
- Provider binaries for Linux, macOS, and Windows on amd64 and arm64, with SHA-256 checksums, an
  SBOM per archive, and a SLSA build provenance attestation for each archive and SBOM (verify one
  with `gh attestation verify <archive> --owner jflavan`).
- Cookie Consent (Customer REST API) calls retry `429` and `503` responses with exponential backoff,
  honoring `Retry-After` (capped at one minute). Reads, updates, deletes, and the publish request also
  retry other `5xx` responses, and reads retry dropped connections. Config and rule creates are not
  replayed after an ambiguous `5xx` response, so a server error never creates a duplicate. The
  `Consent` resource and every function, including `sendSubjectCode` and `verifySubjectCode`, send
  each request once, without retries.
- Every HTTP request identifies the provider with the `pulumi-osano/<version>` user agent.
- Documentation: quickstart examples (TypeScript, Python, Go), a canonical Cookie Consent example
  (C# with a TypeScript companion), an end-to-end workflow guide, importing, upgrade,
  troubleshooting, and state management guides, and Pulumi Registry pages (`docs/_index.md`,
  `docs/installation-configuration.md`).

### Cookie Consent behavior to know

- `pulumi preview`, `pulumi refresh`, and `pulumi import` never publish. A `CookieConsentPublication`
  publishes when it is created and when an update changes its `changeToken`,
  `keepUnclassifiedTattles`, `description`, or `webhookUrl`. The caller manages `changeToken`:
  derive it deterministically (for example a SHA-256 hash) from every publish-relevant value so each
  real change publishes exactly once and an unchanged program is a no-op.
- `keepUnclassifiedTattles` defaults to `true`, so publishing preserves unclassified discoveries
  instead of deleting them.
- Publication waits up to the resource's create/update `customTimeouts`, or 20 minutes when none is
  set. Osano's CDN can take up to 15 more minutes to serve a new revision everywhere.
- `pulumi refresh` reports `publishStatus` as `outdated` after an edit in the Osano dashboard but
  does not publish. An imported publication adopts the token
  `import:<lastPublished>:<publishedRevision>`, so applying the program's own token republishes once.
- Deleting a `CookieConsentRule` deletes it in Osano. Osano has no delete or unpublish endpoint for
  configurations, so destroying a `CookieConsentConfig` or `CookieConsentPublication` only removes it
  from Pulumi state; the configuration and its published script stay live upstream.
- Changing provider configuration, such as rotating an API key, updates the provider in place and
  never replaces the Cookie Consent resources it manages.
- Consent records are immutable in Osano; destroying a `Consent` resource only removes it from
  Pulumi state.

### Deprecated

- The `ucApiKey` and `ucBaseUrl` provider configuration keys. They are still read as fallbacks;
  use `unifiedConsentApiKey` and `apiBaseUrl` instead.

[Unreleased]: https://github.com/jflavan/pulumi-osano/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jflavan/pulumi-osano/releases/tag/v0.1.0
