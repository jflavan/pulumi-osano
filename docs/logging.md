# Logging

## Provider Logs

- The provider does not log request bodies. Sensitive payloads stay in memory until they are sent to Osano.
- Provider errors include the HTTP status code and the response body Osano returned, which is usually enough to identify a validation or authorization failure.
- The provider does not implement dedicated `OSANO_LOG_HTTP` or `OSANO_PROVIDER_DEBUG` flags. Use Pulumi's verbose logging instead:

```bash
pulumi up --logtostderr --logflow -v=9 2> pulumi-debug.log
```

Verbose logs can contain configuration values and resource inputs. Redact API keys, subject identifiers, and verification codes before sharing them.

## Correlating with Osano

Add a custom attribute (for example `attributes["pulumiDeploymentId"]`) so you can correlate Pulumi deployments with entries in Osano's audit log.

Every request the provider sends identifies it with the User-Agent `pulumi-osano/<version>` (for example `pulumi-osano/0.1.0`).

## Troubleshooting Checklist

1. Re-run the failing command with `--logtostderr --logflow -v=9` and capture the output.
2. Record the HTTP status and response body from the provider error, the approximate time of the request, and the provider version (`pulumi plugin ls`), and include them in support tickets.
3. Delete the debug log once the issue is resolved.
