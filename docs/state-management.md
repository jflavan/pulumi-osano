# State Management

Pulumi stores the inputs and outputs for each `osano:index:Consent` resource in the stack state file. Keep the following in mind:

- **Secrets**: mark tokens, subject identifiers, and any PII as secrets via Pulumi config or `pulumi.secret(...)` helpers. Secrets stay encrypted in state and in the Pulumi Service backend.
- **History**: Osano treats consent submissions as append-only events. Deleting a Pulumi resource does not delete the downstream record. Instead it only removes the logical representation from your stack state.
- **Refresh**: `pulumi refresh` replays read operations by calling `/v2/consents/unified/{subject}`. If Osano reports that a subject no longer has any consents, the provider will mark the resource as deleted.
- **Stack separation**: Keep production consent stacks isolated from development/test stacks to avoid mixing API keys or sample data.

Use the Pulumi Service's state diffing and audit logs to understand who triggered each set of consent submissions.
