# State Management

Pulumi stores each resource's inputs and outputs in stack state. Keep production
and development stacks separate so credentials, test subjects, and Cookie
Consent configurations are not mixed.

## Secrets and public installation outputs

Store API keys in encrypted Pulumi configuration:

```bash
pulumi config set osano:osanoApiKey --secret
pulumi config set osano:unifiedConsentApiKey --secret
```

Environment variables `OSANO_API_KEY` and `OSANO_UC_API_KEY` are supported for
CI. Inferred provider API-key properties can appear in provider state, but the
schema marks them `secret: true`; never place credentials in ordinary resource
inputs, outputs, `changeToken`, or diagnostics. Mark subject identifiers and
other PII with Pulumi secret helpers as well.

`CookieConsentPublication.scriptSrc` and `scriptTag` contain only the public
customer/config installation path. They are deliberately non-secret so they can
feed site deployment resources.

## Cookie Consent lifecycle

- `CookieConsentConfig` state stores the inputs as applied plus Osano's
  publication metadata. Refresh surfaces drift in declared values, but the
  `configuration` object tracks only the keys the program declares, so keys
  Osano adds on its side never produce a diff. Its delete is state-only because
  Osano exposes no configuration delete endpoint.
- `CookieConsentRule` uses `<configId>/<ruleId>` identity. Deleting the Pulumi
  resource deletes the managed rule from Osano; a missing upstream rule is
  treated as already deleted.
- `CookieConsentPublication` stores the desired publication options and
  caller-managed `changeToken` along with observed status, revision, and script
  outputs. Changing the token or an option causes an update and republish;
  unchanged inputs are a no-op.
- Publication delete is state-only because Osano exposes no unpublish endpoint.
  The upstream configuration and previously published script remain active.
- Provider configuration changes (API key rotation, `requestTimeoutSeconds`,
  base URLs) update the provider in place and never replace the resources it
  manages.

Build `changeToken` deterministically from every publish-relevant desired
configuration and rule value. Do not hash API keys or depend on unstable map
ordering. Explicit dependencies ensure the publication waits for configuration
and rule updates; the token ensures those desired-state changes trigger an
update.

## Refresh, preview, import, and destroy

- `pulumi preview` does not publish or otherwise mutate the Customer REST API.
- `pulumi refresh` only reads. It can expose an externally changed publication
  as `outdated` but never republishes it.
- Import only reads. Publication import adopts
  `import:<lastPublished>:<publishedRevision>`; applying a program with a
  different intended token may cause one controlled republish.
- `pulumi destroy` deletes managed rules, but configuration/publication removal
  is state-only. Immutable Unified Consent submissions also remain upstream.

For Unified Consent resources, refresh checks that the subject still has
consent and updates `lastSynced`; it keeps the submitted inputs rather than the
subject's merged consent view, so a refresh never triggers a replacement.
Consent submissions are append-only events, so removing a logical Pulumi
resource never erases the historical Osano record.

Use Pulumi state diffs and audit logs to understand who applied each desired
state. Review retained Osano configurations separately after stack teardown.
