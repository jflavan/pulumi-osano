# Logging

## Provider Logs

- The provider does not log request bodies by default. Sensitive payloads stay in-memory until transmitted to Osano.
- Set `PULUMI_DEBUG_COMMANDS=1` when running `pulumi up` to see verbose logs, including HTTP response codes.
- The provider does not currently implement dedicated `OSANO_LOG_HTTP` or `OSANO_PROVIDER_DEBUG` environment flags. Use Pulumi debug logging when diagnosing request failures.

## Correlating with Osano

We recommend adding a custom attribute (for example `attributes["pulumiDeploymentId"]`) so you can correlate Pulumi deployments with entries inside Osano's audit log.

## Troubleshooting Checklist

1. Enable Pulumi debug logging.
2. Capture the failing request ID from the Osano response and include it in support tickets.

Remove the environment variables once the issue is resolved to avoid noisy logs.
