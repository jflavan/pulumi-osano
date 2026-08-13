# Upgrade Guide

This guide captures breaking changes and migration tips between provider versions.

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
- Publication still needs a separate Pulumi create/update custom timeout; use
  twenty minutes for Osano's asynchronous operation.

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

## 0.x → 0.y

The provider is still pre-1.0, so we may ship breaking changes between minor releases. Always pin your Pulumi program to an explicit version in `Pulumi.yaml`:

```yaml
plugins:
  providers:
    - name: osano
      version: 0.2.0
```

### Consent resource shape changes

- Fields may be renamed as Osano expands the API. Review the release notes for each version and update your Pulumi code accordingly.
- When new required fields are added, run `pulumi preview` to spot the diff before applying.

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
