# End-to-End Workflow

This guide walks through the complete lifecycle for both provider areas, from
installation to teardown, and the contributor loop used to change and validate
the provider. Each step links to the detailed reference page.

## 1. Choose keys and install

| Workload | Key | Config / environment |
| --- | --- | --- |
| Cookie Consent resources, `sendSubjectCode`, `verifySubjectCode` | Osano Customer REST API key (`x-osano-api-key`) | `osano:osanoApiKey` (secret) or `OSANO_API_KEY` |
| `Consent` resource and the other Unified Consent functions | Unified Consent API key (`x-uc-api-key`) | `osano:unifiedConsentApiKey` (secret) or `OSANO_UC_API_KEY` |

Environment variables take precedence over stack config. Store config values
with `--secret`:

```bash
pulumi config set osano:osanoApiKey --secret
pulumi config set osano:unifiedConsentApiKey --secret
```

**Published packages.** Add the SDK for your language (see the
[README](../README.md#installation)). The SDK declares its provider plugin, and
Pulumi downloads the matching release from GitHub on the first `pulumi preview`
or `pulumi up`; no manual plugin install is needed.

**From a clone.** The generated SDKs request the development plugin version
`1.0.0-alpha.0+dev`, which is never published, so build and install the local
provider binary before running any repo-local example:

```bash
mise exec -- make provider
mise exec -- pulumi plugin install resource osano 1.0.0-alpha.0+dev \
  --file ./bin/pulumi-resource-osano --exact --reinstall
```

## 2. Cookie Consent: first deployment

The canonical program is [examples/cookie-consent](../examples/cookie-consent)
(C#, with a TypeScript companion). It declares three kinds of resources:

1. `CookieConsentConfig`: name, domains, mode, and the CMP `configuration`
   object (which must include `storagePolicyHref`).
2. One `CookieConsentRule` per cookie, script, iframe, or localStorage pattern,
   each referencing `consentConfig.configId`.
3. One `CookieConsentPublication` that publishes the config and returns the
   install script. Give it:
   - `dependsOn` every config and rule resource, so it publishes only after
     they settle;
   - a `changeToken` derived deterministically (for example a SHA-256 hash)
     from every publish-relevant value, built from the same data the config
     and rules use;
   - `customTimeouts` of 20 minutes for create and update; the provider waits
     for that window and stops after 20 minutes when none is set.

Deploy:

```bash
export OSANO_API_KEY="replace-with-a-customer-rest-api-key"
pulumi config set domain example.com
pulumi config set storagePolicyHref https://example.com/privacy/cookies
pulumi preview   # makes no changes in Osano
pulumi up        # creates the config and rules, then publishes and waits
pulumi stack output cookieConsentScriptTag
```

Install the returned tag first in your site's `<head>`, with no `async` or
`defer` attribute:

```html
<script src="https://cmp.osano.com/CUSTOMER_ID/CONFIG_ID/osano.js"></script>
```

Osano's CDN can take up to 15 minutes after publication to serve the new
revision everywhere.

## 3. Cookie Consent: day-2 changes

1. Edit a rule or configuration value in the program. Because the token is
   derived from the same values, it changes too.
2. `pulumi preview` shows in-place updates for the edited config/rule and for
   the publication's `changeToken`.
3. `pulumi up` applies the edits, then republishes exactly once.
4. Running `pulumi up` again with no edits is a no-op and does not publish.

Changing `configId` or a rule's `storeType` replaces that resource. Changing the
publication's `keepUnclassifiedTattles`, `description`, or `webhookUrl` also
republishes. `keepUnclassifiedTattles` defaults to `true` so publication does not
delete unclassified discoveries. Changing provider configuration, such as
rotating the API key or adding `osano:requestTimeoutSeconds`, updates the
provider in place and never replaces the Cookie Consent resources.

If someone edits the configuration in the Osano dashboard, `pulumi refresh`
reports the publication's `publishStatus` as `outdated` but never publishes.
Reconcile the program, change the token deliberately, and run `pulumi up` once.
See [state management](state-management.md) and
[troubleshooting](troubleshooting.md#persistent-outdated-status).

## 4. Adopt existing Cookie Consent resources

Write the program to match the remote values first, then import:

```bash
pulumi import osano:index:CookieConsentConfig consentConfig <configId>
pulumi import osano:index:CookieConsentRule analyticsRule <configId>/<ruleId>
pulumi import osano:index:CookieConsentPublication publication <configId>
```

Imports only read. An imported publication adopts the token
`import:<lastPublished>:<publishedRevision>`, so applying your program's own
token causes one controlled republish. Details: [IMPORTING.md](IMPORTING.md).

## 5. Tear down

`pulumi destroy` deletes managed rules in Osano. Osano has no delete or
unpublish endpoint for configurations, so destroying the config and publication
only removes them from Pulumi state: the configuration and its published script
stay live. Remove the script tag from your site and disable the configuration in
Osano if it should stop serving.

## 6. Unified Consent

Submit a consent decision with the `Consent` resource (see
[examples/quickstart](../examples/quickstart)):

```ts
const consent = new osano.Consent("example", {
    subject: { verifiedId: subjectRef },
    actions: [{ target: privacyProtocolId, vendor: configId, action: "ACCEPT" }],
    origin: "api",
});
```

Read the subject's merged state back with a function:

```ts
const unified = osano.getUnifiedConsentOutput({ subjectRef });
export const hasConsent = unified.exists;
```

Behavior to plan for:

- Consent records are immutable. Destroy only forgets the Pulumi resource, and
  records cannot be imported.
- `pulumi refresh` confirms the subject still has consent and updates
  `lastSynced`; it keeps your inputs, so a refresh never forces a new
  submission. If Osano reports no consent for the subject, the resource drops
  from state and the next `pulumi up` submits it again.
- Pulumi runs functions on every preview, update, and refresh. That is fine for
  the `get*` and `checkConsent` lookups, but `sendSubjectCode` would send a new
  code each run and `verifySubjectCode` would reuse a one-time code. Run
  subject verification from a dedicated short-lived program or directly from
  your application, not from a long-lived stack.

## 7. Contributor loop

```bash
eval "$(mise activate zsh)" && mise install   # pinned toolchain
make codegen                # after any provider/ change: schema + all SDKs
make lint
make test_provider          # mocked HTTP, no credentials
make build_examples         # C#/TypeScript Cookie Consent + Go quickstart
make test_e2e_compile       # vets every live e2e suite without credentials
```

To exercise a change against Osano, install the local plugin (section 1), then
run an example with `pulumi preview` and, only when you intend to create real
resources, `pulumi up`.

The opt-in suites under [tests/e2e](../tests/e2e) call Osano's Unified Consent
and subject-verification APIs directly (not through the provider) to confirm
the upstream contract the provider relies on; see
[tests/README.md](../tests/README.md). The Cookie Consent lifecycle is covered
by the provider's mocked-HTTP tests plus the opt-in example deployment above.

## Troubleshooting index

| Symptom | Where to look |
| --- | --- |
| `Osano API key not configured` / `Unified Consent API key not configured` | [troubleshooting](troubleshooting.md#missing-customer-rest-api-key) |
| Publication `409`, `429`, `error`, or timeout | [troubleshooting](troubleshooting.md#publication-status-and-http-responses) |
| Create failed with `500`/`502`/`504` | [troubleshooting](troubleshooting.md#cookie-consent-create-failed-with-a-server-error) |
| Script still serves the old revision | [troubleshooting](troubleshooting.md#delayed-cdn-propagation-after-published) |
| Plugin `osano` not found when running from a clone | Section 1 of this guide |
| Verbose request logging | [logging](logging.md) |
