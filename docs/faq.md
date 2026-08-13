# Frequently Asked Questions

**Is this an official Osano integration?**
> No. This is a community-maintained Pulumi provider built against Osano's public APIs. It is not affiliated with Osano.

**Which APIs are supported?**
> The provider manages Cookie Consent configurations, rules, and explicit publications through the [Customer REST API](https://developers.osano.com/customer-rest-api). It also supports Unified Consent submissions and helper invokes for consent lookups, subject resolution, configuration and collection reads, consent profiles, and subject verification.

**Do I need both API keys?**
> Only for mixed workloads. Cookie Consent resources and administrative subject/profile routes require the Customer REST API key (`OSANO_API_KEY` or secret provider config `osano:osanoApiKey`). Unified Consent collection routes require `OSANO_UC_API_KEY` or secret provider config `osano:unifiedConsentApiKey`.

**What causes Cookie Consent to publish?**
> Creating `CookieConsentPublication` queues publication after its declared dependencies. Changing its `changeToken` or publication options queues one in-place republish; an unchanged update does not publish. Include every publish-relevant desired configuration and rule value in a stable token. [`dependsOn`](https://www.pulumi.com/docs/iac/concepts/resources/options/dependson/) controls ordering but does not itself trigger an update, so both the dependencies and token are required.

**Do preview, refresh, or import publish?**
> No. Preview performs no Customer REST API mutation, and refresh/import only read. If external changes make the configuration `outdated`, `pulumi refresh` exposes that status without publishing. A later input change, such as the program's intended `changeToken` replacing an import adoption token, may cause one publication during `pulumi up`.

**How do I install the returned CMP script?**
> `scriptSrc` and `scriptTag` are deliberately public deployment outputs. Place `scriptTag` first in the site `<head>`, with no `async` or `defer`, so Osano loads before scripts it may control. See the [Consent JavaScript API](https://developers.osano.com/cmp/javascript-api/developer-documentation-consent-javascript-api).

**What does destroy remove from Osano?**
> Managed Cookie Consent rules are deleted upstream. Osano exposes no delete/unpublish endpoint for configurations or publications, so those deletes remove only Pulumi state; the upstream configuration and published script remain active. Immutable Unified Consent events also remain in Osano.

**Can I import existing consent records?**
> Immutable Unified Consent records are not importable. Cookie Consent configurations, composite-ID rules, and publications are importable; see [IMPORTING.md](./IMPORTING.md) for exact commands and adoption-token behavior.

**Does this provider store personal data or credentials in state?**
> Pulumi stores resource inputs and outputs. Mark subject IDs, tags, attributes, and other PII as secrets. Provider API-key configuration inferred into provider state is marked `secret: true`; keep source config encrypted with `pulumi config set --secret`. Public CMP `scriptSrc` and `scriptTag` outputs intentionally are not secrets.

**How are API errors surfaced?**
> Diagnostics identify the operation, status, and relevant resource ID without including API keys. Use the status-specific guidance in [troubleshooting.md](./troubleshooting.md).

**Where can I ask more questions?**
> Open a GitHub Discussion or an issue. Never share real subject identifiers or API keys.
