# Troubleshooting

## Common Errors

### `Unified Consent API key not configured`
Set `pulumi config set osano:unifiedConsentApiKey --secret` or export `OSANO_UC_API_KEY`. The provider requires this key for `/v2/consents` and `/v2/consents/unified` routes.

### `Osano API key not configured`
Subject/profile helper invokes require the standard Osano API key. Provide it via `osano:osanoApiKey` or `OSANO_API_KEY`.

### `400 Bad Request`
Osano returns 400 when the subject reference is unknown or the consent payload violates schema rules. Check the Pulumi diagnostic for the response body; it is surfaced verbatim. Fix invalid IDs or missing fields and rerun `pulumi up`.

### `401 Unauthorized`
Verify that the API key has not expired and matches the environment (production vs. sandbox). Rotate the key inside Pulumi config and rerun the deployment.

### `429 Too Many Requests`
You hit the per-key rate limit. Split large consent submissions across multiple stacks or add sleeps between `pulumi up` executions. Also confirm that examples/tests are not hammering production keys.

## Debug Strategy

1. Re-run the command with `PULUMI_LOGGING_OVERRIDE=debug` to capture detailed diagnostics.
2. Use the `getUnifiedConsent` invoke to inspect the subject's current state; this helps confirm whether a previous submission succeeded.
3. If an error persists, capture the Pulumi log plus the Osano response body and open an issue.

## Contact

- GitHub Issues for bugs
- GitHub Discussions for questions or feature requests

Never share API keys or real subject identifiers in public threads.
