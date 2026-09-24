# Performance Notes

The provider is lightweight: it serializes Pulumi inputs and calls the Osano REST APIs. There are still a few considerations when running it at scale.

## Throughput

- Osano enforces rate limits per API key. Batch multiple consent actions into a single `Consent` resource when possible to minimize calls.
- Cookie Consent calls retry `429` and `503` responses with bounded backoff (honoring `Retry-After`, capped at one minute per wait). Reads, updates, deletes, and the publish request also retry other `5xx` responses, and reads retry dropped connections; config and rule creates do not, so an ambiguous server error never creates a duplicate config or rule.
- Separate workloads into distinct stacks (for example, `consents-eu`, `consents-us`) to avoid throttling large previews.

## Latency

- The default HTTP timeout is 60 seconds per request. Override it with `osano:requestTimeoutSeconds` or `OSANO_API_TIMEOUT_SECONDS`.
- The Pulumi engine runs independent resources in parallel (`pulumi up --parallel` controls the limit). Use `dependsOn` only where ordering matters, such as a `CookieConsentPublication` that must wait for its rules.
- `CookieConsentPublication` waits for Osano to finish publishing, which can take several minutes. It honors the resource's create/update `customTimeouts` and stops after 20 minutes when none is set; the examples set 20 minutes explicitly.

## State Size

- Each `osano:index:Consent` resource stores its inputs plus `consentId` and `lastSynced`. Keep attributes compact to avoid bloating state snapshots.
- Use stack outputs sparingly; export aggregated values instead of large payloads.

## Preview Optimization

During `pulumi preview`, resources make no outbound HTTP calls: create previews report server-assigned outputs as unknown, and update previews carry the prior outputs forward. Functions (invokes) are different: Pulumi runs them during preview, so every `get*` lookup, and any `sendSubjectCode` or `verifySubjectCode` call, contacts Osano on each preview.
