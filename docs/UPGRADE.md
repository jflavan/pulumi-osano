# Upgrade Guide

This guide captures breaking changes and migration tips between provider versions.

## Review hardening release

These fixes change behavior without changing resource tokens:

- `Consent` refresh keeps the submitted inputs and only updates `lastSynced`.
  Earlier builds copied the subject's merged Unified Consent view into the
  inputs, so the next `pulumi up` replaced the resource and submitted a
  duplicate consent. Run `pulumi preview` after upgrading. If a stack refreshed
  by an older build shows a `Consent` replacement caused only by `actions` or
  `attributes`, applying it would submit a new consent record; to keep the
  existing record instead, add `ignoreChanges: ["actions", "attributes"]` to
  that resource.
- Unified Consent requests keep any path prefix on `osano:apiBaseUrl` or
  `OSANO_API_BASE_URL` (for example a proxy mounted at `/osano`). Earlier builds
  sent `/v2/...` to the host root.
- Cookie Consent config and rule creates are no longer retried after ambiguous
  `500`/`502`/`504` responses. `429` and `503` are still retried, and
  publication requests still retry every retryable status.
- `verifySubjectCode.code` is now a secret input.
- `CookieConsentConfig` preview accepts inputs that are unknown until apply, and
  rule limits count characters rather than bytes.
- Update previews keep `configId`, `customerId`, `ruleId`, `scriptSrc`, and
  `scriptTag` known. Earlier builds marked them unknown whenever any input
  changed, so editing a config previewed every dependent rule and publication as
  a replacement (the real update did not replace them).
- `Consent` validation runs in `Check` and skips inputs that are unknown until
  apply, so a `vendor` or `target` wired from another resource's output no longer
  fails preview.
- Cookie Consent GET requests retry transport failures (for example a dropped
  connection during a long publication wait); writes still never replay after a
  transport failure.
- Provider configuration changes (rotating `osano:osanoApiKey`, adding
  `osano:requestTimeoutSeconds`, or changing a base URL) now update the provider
  in place. Earlier builds reported every provider config change as a
  replacement, so Pulumi replaced every Cookie Consent config, rule, and
  publication and created duplicate Osano configurations. The same defect made
  the first `pulumi up` after `pulumi import` replace the imported resources.
- `CookieConsentConfig` and `CookieConsentRule` state now stores inputs as
  applied. Keys Osano adds to `configuration` and server defaults for optional
  rule fields the program leaves unset no longer produce a diff on every
  `pulumi up`; refresh still surfaces drift in declared values.
- Config create and update omit `orgIds` unless it is set and send an empty
  list only to clear a previously managed value, instead of sending `null`.
- A `CookieConsentPublication.webhookUrl` wired from another resource's output
  no longer fails preview while it is unknown.

## Cookie Consent publication release

This release adds `osano:index:CookieConsentPublication` without changing the
tokens of existing resources. The resource queues asynchronous Cookie Consent
publication after configuration/rule dependencies settle, waits for completion,
and returns public `scriptSrc` and `scriptTag` installation outputs. It requires
a deterministic caller-managed `changeToken`; unchanged inputs do not publish.
Preview, refresh, import, and delete never publish.

`CookieConsentRule` adds optional `ruleType`, `description`, and `expiry` fields.
`description` and `expiry` apply only to cookie rules. Removed nullable fields
are now cleared upstream, list reads paginate, and missing rules/configurations
are treated as deleted state.

### Customer REST credentials and timeouts

Cookie Consent resources now share the Customer REST configuration path:

- `OSANO_API_KEY` takes precedence over secret `osano:osanoApiKey`.
- `osano:customerBaseUrl` defaults to `https://api.osano.com`.
- A valid positive `OSANO_API_TIMEOUT_SECONDS` takes precedence over
  `osano:requestTimeoutSeconds`; otherwise provider config/default 60 seconds is
  used for individual Customer REST and Unified Consent HTTP calls.
- Publication waits for the resource's Pulumi create/update `customTimeouts`
  and stops after twenty minutes when none is set; the examples set twenty
  minutes explicitly.

If credentials were previously supplied only for administrative routes, verify
the same key is authorized for Customer REST CMP operations before applying.

### Rule identities and adoption

New and imported rule state uses composite `<configId>/<ruleId>` identities:

```bash
pulumi import osano:index:CookieConsentRule analyticsRule <configId>/<ruleId>
```

Existing tracked numeric rule IDs remain readable with their state-held
`configId` and normalize on refresh without replacing the upstream rule. Review
the preview after upgrading, especially if state was manually edited.

Publication import uses the config ID and adopts
`import:<lastPublished>:<publishedRevision>` without publishing. Replacing that
adoption token with the program's intended token may cause one controlled
republish.

### Delete behavior

`CookieConsentRule` deletion is a real upstream delete. Osano exposes no
delete/unpublish endpoint for Cookie Consent configurations or publications, so
deleting either of those resources removes only Pulumi state. A destroyed stack
therefore retains the upstream configuration and published script; review and
disable retained customer resources in Osano separately when required.

## Pinning alpha releases

The provider is pre-1.0 (`1.0.0-alpha.*`), so breaking changes can ship between
alpha builds. Pin the SDK package to an exact version in your program's
dependency manifest (`package.json`, `requirements.txt`, `.csproj`, or `go.mod`)
and review the release notes before upgrading. The Pulumi engine installs the
matching provider plugin for the pinned SDK version.

- Fields may be renamed as Osano expands the API. Review the release notes for
  each version and update your Pulumi code accordingly.
- When new required fields are added, run `pulumi preview` to spot the diff
  before applying.

### SDK namespace changes

If you consume the generated SDKs directly:

- Node.js: `import * as osano from "@jflavan/pulumi-osano";`
- Python: `import pulumi_osano as osano`
- Go: `github.com/jflavan/pulumi-osano/sdk/go/osano`

Check your lockfiles to ensure the new version is installed.

### Rolling out upgrades safely

1. Run `make codegen && make build_sdks` locally to verify that generation still works.
2. Compile resource examples with `make build_cookie_consent_examples`.
3. Update your example projects and run `pulumi preview`.
4. Deploy to a staging stack before touching production.

Report regressions as GitHub issues with stack traces and the Osano API response if available.
