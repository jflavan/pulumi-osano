# Importing Existing Osano Resources

## Cookie Consent resources

Existing Cookie Consent configurations, rules, and publications can be adopted
with these exact ID formats:

```bash
pulumi import osano:index:CookieConsentConfig consentConfig <configId>
pulumi import osano:index:CookieConsentRule analyticsRule <configId>/<ruleId>
pulumi import osano:index:CookieConsentPublication publication <configId>
```

A configuration import reads the remote configuration into state. It does not
publish. A rule import requires the composite `<configId>/<ruleId>` identity;
older tracked rules with numeric IDs remain readable, and refresh normalizes
their identity without replacing the upstream rule.

A publication import also performs only a read and never queues publication.
Because Osano cannot reconstruct the caller's prior desired-state token, the
provider adopts the observed publication with:

```text
import:<lastPublished>:<publishedRevision>
```

After adding the imported resource to a program, supply the program's intended
deterministic `changeToken`. If it differs from the adoption token, the first
`pulumi up` may perform one controlled republish. Include every
publish-relevant configuration and rule value in the token, but do not include
API keys or other secrets.

Before importing, ensure the program models the remote values and run
`pulumi preview`. Import and preview do not mutate Osano.

## Unified Consent records

The Osano Unified Consent API records immutable consent events rather than
mutable resources. There is no canonical consent resource that can be imported
after it has been recorded inside Osano.

- `osano:index:Consent` represents a single API submission. Osano does not
  expose identifiers for individual consent actions that map back to Pulumi,
  so these immutable records are not importable.
- Adopt Pulumi by submitting new desired consent events. Historical records
  remain untouched and queryable in Osano.
- Use `osano:index:getUnifiedConsent` to read the latest view for a subject and
  seed automations without importing a record.

For an importing scenario not covered here, open a GitHub Discussion without
including real subject identifiers or API keys.
