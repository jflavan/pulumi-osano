# Frequently Asked Questions

**Is this an official Osano integration?**
> No. This is a community-maintained Pulumi provider built against Osano's public APIs. It is not affiliated with Osano.

**Where is the provider published? Is it in the Pulumi Registry?**
> The SDKs are published to npm (`@jflavan/pulumi-osano`), PyPI (`pulumi-osano`), NuGet (`Community.Pulumi.Osano`), Maven Central (`io.github.jflavan.pulumi:pulumi-osano`), and the Go module proxy (`github.com/jflavan/pulumi-osano/sdk/go/osano`). The provider plugin is attached to each [GitHub release](https://github.com/jflavan/pulumi-osano/releases) and downloads automatically. The provider is not listed in the Pulumi Registry yet. See [PUBLISHING.md](./PUBLISHING.md) for install commands and for verifying checksums, signatures, and provenance.

**Which APIs are supported?**
> The provider manages Cookie Consent configurations, rules, and explicit publications through the [Customer REST API](https://developers.osano.com/customer-rest-api), and reads configurations, rules, discoveries, and the audit log with the `getCookieConsent*` functions. It also supports Unified Consent submissions (including Global Privacy Control consents) and functions for consent lookups, subject, profile, and session resolution, configuration and collection reads, consent profiles, and subject verification. Osano operations that do not fit declarative infrastructure as code, such as merging subjects, creating subject profiles, and creating tokens, are not exposed.

**Do I need both API keys?**
> Only for mixed workloads. Cookie Consent resources and functions require the Customer REST API key (`OSANO_API_KEY` or secret provider config `osano:osanoApiKey`). The `Consent` resource and the other Unified Consent functions require `OSANO_UC_API_KEY` or secret provider config `osano:unifiedConsentApiKey`. `sendSubjectCode` and `verifySubjectCode` use the Customer REST API key when it is set and otherwise the Unified Consent API key.

**What causes Cookie Consent to publish?**
> Creating `CookieConsentPublication` queues publication after its declared dependencies. Changing its `changeToken` or publication options queues one in-place republish; an unchanged update does not publish. Include every publish-relevant desired configuration and rule value in a stable token. [`dependsOn`](https://www.pulumi.com/docs/iac/concepts/resources/options/dependson/) controls ordering but does not itself trigger an update, so both the dependencies and token are required.

**Do preview, refresh, or import publish?**
> No. Preview performs no Customer REST API mutation, and refresh/import only read. If external changes make the configuration `outdated`, `pulumi refresh` exposes that status without publishing. A later input change, such as the program's intended `changeToken` replacing an import adoption token, may cause one publication during `pulumi up`.

**How do I install the returned CMP script?**
> `scriptSrc` and `scriptTag` are deliberately public deployment outputs. Pass `scriptTag` to the resource that renders or configures the site and place it first in the site `<head>`, with no `async` or `defer`, so Osano loads before scripts it may control. The examples export a `headHtml` fragment that does this. See [End-to-End Workflow, section 3](./end-to-end-workflow.md#3-hand-the-script-to-the-website) and the [Consent JavaScript API](https://developers.osano.com/cmp/javascript-api/developer-documentation-consent-javascript-api).

**How does a stack that does not manage the configuration get the script?**
> Read the consent stack's `cookieConsentScriptTag` output with a stack reference, or call `getCookieConsentConfig` with the config ID, which also works for configurations managed in the Osano dashboard and returns the publish status. The script URL returns `403` until the configuration's first publication; `lastPublished` is `0` until then.

**How long until a publication reaches visitors?**
> `CookieConsentPublication` completes when Osano reports the configuration as published. Osano's CDN can then take up to 15 minutes to serve the new revision, and browsers cache `osano.js` for 24 hours, so a returning visitor can see the previous revision for up to a day. The script URL does not change between revisions.

**What does destroy remove from Osano?**
> Managed Cookie Consent rules are deleted upstream. Osano exposes no delete/unpublish endpoint for configurations or publications, so those deletes remove only Pulumi state; the upstream configuration and published script remain active. Immutable Unified Consent events also remain in Osano.

**Can I import existing consent records?**
> Immutable Unified Consent records are not importable. Cookie Consent configurations, composite-ID rules, and publications are importable; see [IMPORTING.md](./IMPORTING.md) for exact commands and adoption-token behavior.

**Does this provider store personal data or credentials in state?**
> Pulumi stores resource inputs and outputs. Mark subject IDs, tags, attributes, and other PII as secrets. Provider API-key configuration inferred into provider state is marked `secret: true`; keep source config encrypted with `pulumi config set --secret`. The schema also marks `CookieConsentPublication.webhookUrl`, `Consent.sessionToken`, the subject verification `code`, `session`, `destination`, and `verifySubjectCode.profile` values, and the personal-data outputs of `getSubjectProfile` and `getSession` as secrets. Public CMP `scriptSrc` and `scriptTag` outputs intentionally are not secrets. See [state management](./state-management.md#secrets-and-public-installation-outputs).

**How are API errors surfaced?**
> Publication terminal-status diagnostics contain `status`, `lastPublished`, and `publishedRevision`; they contain neither the config ID nor the API key. Other request diagnostics vary by operation. Use the status-specific guidance in [troubleshooting.md](./troubleshooting.md).

**Why did `sendSubjectCode` send a code on `pulumi preview`?**
> Pulumi runs functions (invokes) during every preview, update, and refresh. Declaring `sendSubjectCode` in a stack therefore sends a new code each run, and `verifySubjectCode` re-submits a one-time code that is already used. Call these functions from automation code rather than from a long-lived stack. `verifySubjectCode.code` is marked secret. To verify an SMS code, pass the `session` that `sendSubjectCode` returns to `verifySubjectCode`, which requires it with `phone`.

**Does refresh change my `Consent` resource?**
> Refresh confirms the subject, identified by its `verifiedId` or `anonymousId`, still has Unified Consent and updates `lastSynced`. It keeps the submitted inputs, because the unified view merges every consent for the subject and cannot be mapped back to one submission. If Osano reports no consent for the subject, refresh removes the resource from state and the next `pulumi up` submits it again.

**Why does `pulumi up --refresh` update my Cookie Consent configuration?**
> Refresh compares only the configuration keys your program declares, recursively into nested objects such as `palette`, so with no edits in the program or in Osano a refreshed run is a no-op. `variantMapping` is compared as a whole, and `ccpaRelaxed` next to a non-empty `variantMapping` drifts because Osano rewrites it. See [troubleshooting](./troubleshooting.md#drift-or-an-update-on-every-refresh).

**Where can I ask more questions?**
> Open a [GitHub issue](https://github.com/jflavan/pulumi-osano/issues). Never share real subject identifiers or API keys. Report vulnerabilities as described in [SECURITY.md](../SECURITY.md).
