# Importing Existing Osano Resources

The Osano Unified Consent API records immutable consent events rather than mutable resources. That means there is no canonical "consent resource" that can be imported into Pulumi after it has been recorded inside Osano. Instead, Pulumi programs typically replay the desired consent payload with the latest configuration so that the history stored in Osano matches the infrastructure definition.

## Consent Records

- `osano:index:Consent` represents a single API submission (for example, a visitor updating preferences). Importing those records is not supported because Osano does not expose identifiers for individual consent actions that can be mapped back to Pulumi resources.
- When adopting Pulumi for existing environments, we recommend creating logical representations (for example, a Pulumi stack per product or brand) and using the provider to submit **new** events. Historical records remain untouched in Osano and continue to be queryable.

## Configuration Objects

Future roadmap versions of this provider may expose additional configuration resources (privacy protocols, collections, or configurations). Once those resources are available you will be able to import them with `pulumi import osano:index:ResourceName identifier`. Each resource will document the exact identifier format (typically the Osano-internal UUID).

## Recommended Approach Today

1. **Model the desired steady state** – describe the consent actions or helper functions that your applications should perform going forward.
2. **Leave historical data in place** – Osano keeps authoritative history; Pulumi only submits future state transitions.
3. **Use invokes for discovery** – the `osano:index:getUnifiedConsent` function lets you read the latest view for a subject so you can seed your automations without importing anything.

If you have a concrete importing scenario we have not covered, open a GitHub Discussion with the details. It will help prioritize future resource coverage.
