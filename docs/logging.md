# Logging

## Provider Logs

- The provider does not log request bodies. Sensitive payloads stay in memory until they are sent to Osano.
- Provider errors include the HTTP status code and the response body Osano returned, which is usually enough to identify a validation or authorization failure.
- The provider does not implement dedicated `OSANO_LOG_HTTP` or `OSANO_PROVIDER_DEBUG` flags. Use Pulumi's verbose logging instead:

```bash
pulumi up --logtostderr --logflow -v=9 2> pulumi-debug.log
```

Verbose logs can contain configuration values and resource inputs. Redact API keys, subject identifiers, verification codes and sessions, and the publication `webhookUrl` before sharing them.

`CookieConsentConfig` reports configuration problems that do not stop a deployment, such as unknown or deprecated configuration keys, as Pulumi warnings in the normal `pulumi preview` and `pulumi up` output; no verbose logging is needed to see them.

## Correlating with Osano

For Unified Consent, add a custom attribute (for example `attributes["pulumiDeploymentId"]`) to each `Consent` so you can correlate Pulumi deployments with consent records in Osano.

For Cookie Consent, `getCookieConsentAuditLog` returns Osano's audit events with their type (for example `cmp.configPublished` or `cmp.configUpdated`), actor, and timestamp. Match a publication or configuration change against the time of the Pulumi update that made it, or find edits made in the Osano dashboard.

Every request the provider sends identifies it with the User-Agent `pulumi-osano/<version>` (for example `pulumi-osano/0.2.0`).

## Troubleshooting Checklist

1. Re-run the failing command with `--logtostderr --logflow -v=9` and capture the output.
2. Record the HTTP status and response body from the provider error, the approximate time of the request, and the provider version (`pulumi plugin ls`), and include them in support tickets.
3. Delete the debug log once the issue is resolved.
