# Troubleshooting

## Missing Unified Consent API key

`Unified Consent API key not configured` means a Unified Consent route needs
`osano:unifiedConsentApiKey` or `OSANO_UC_API_KEY`:

```bash
pulumi config set osano:unifiedConsentApiKey --secret
```

## Missing Customer REST API key

`Osano API key not configured` means a Cookie Consent configuration, rule, or
publication—or a subject send-code/verify function—needs the Osano API key. Set encrypted config or the environment variable:

```bash
pulumi config set osano:osanoApiKey --secret
# or
export OSANO_API_KEY="replace-with-a-customer-rest-api-key"
```

When both are present, `OSANO_API_KEY` takes precedence. Confirm the key belongs
to the same customer/environment as the target config. See the
[Customer REST API](https://developers.osano.com/customer-rest-api).

## Provider plugin not found or not downloaded

Each published SDK names its provider plugin version and download location
(`github://api.github.com/jflavan/pulumi-osano`), and Pulumi downloads the
matching `pulumi-resource-osano` archive from the
[GitHub release](https://github.com/jflavan/pulumi-osano/releases) on the first
`pulumi preview` or `pulumi up`.

- **From a repository clone**: the committed SDKs request the development
  version `0.1.0-alpha.0+dev`, which has no GitHub release, so the download
  fails with `404 HTTP error fetching plugin`. Build and install the local
  plugin as described in
  [End-to-End Workflow, section 1](end-to-end-workflow.md#1-choose-keys-and-install).
- **GitHub rate limit**: plugin downloads use the GitHub API. If Pulumi reports
  `GitHub rate limit exceeded`, set `GITHUB_TOKEN` to a GitHub token and run the
  command again.
- **No network access at deploy time**: install the plugin in advance, at the
  same version as the SDK:

  ```bash
  pulumi plugin install resource osano 0.1.0 --server github://api.github.com/jflavan/pulumi-osano
  ```

To verify a downloaded archive's checksum and build provenance, see
[PUBLISHING.md](PUBLISHING.md).

## Publication status and HTTP responses

Osano reports `unpublished`, `in-progress`, `published`, `outdated`, or `error`.
`CookieConsentPublication` waits for a newly accepted operation to reach
`published`; it does not accept stale pre-publish metadata as completion.

### Publication `409 Conflict`

A publish POST can return `409` when the configuration already has a publication
in progress. The provider treats that operation as accepted elsewhere, does not
send another immediate POST, and joins the polling path. Let the current
`pulumi up` continue. If it later times out, check Osano before retrying so the
accepted operation has time to finish.

### Publication `429 Too Many Requests`

A publish POST can return `429` because of rate or publication-capacity limits.
The provider honors `Retry-After` when supplied and otherwise uses bounded
backoff. If the bounded retries are exhausted, wait for the indicated window or
for capacity to recover before running a single new `pulumi up`. Do not run
repeated immediate publish attempts; they extend throttling and can compete with
an already accepted operation.

This publication-specific guidance differs from batching ordinary consent
submissions. Avoid concurrent publication resources for the same config.

### Terminal publication `error`

`error` is an Osano terminal status, so the provider stops instead of continuing
to poll. Open the configuration in Osano, inspect its publication/configuration
validation details, and correct the configuration or rules. Then change the
publish-relevant desired state (and therefore `changeToken`) and run one new
`pulumi up`. The terminal diagnostic contains `status`, `lastPublished`, and
`publishedRevision` from the current response (the metadata values are zero when
Osano does not provide them). It contains neither the config ID nor the API key.

If the configuration was already in `error` before the publish request, Osano
may keep reporting that same `error` until it starts the new operation. The
provider fails on the sixth unchanged poll (about 25 seconds of backoff plus
request time) with `Osano did not start a new publication`, rather than waiting
for the full publication timeout.

### Publication timeout or cancellation

The provider waits for the resource's Pulumi create/update `customTimeouts` and
stops after twenty minutes when none is set. Set twenty minutes explicitly so
the intent is visible in the program, as the canonical C# example does:

```csharp
var publicationOptions = new CustomResourceOptions
{
    CustomTimeouts = new CustomTimeouts
    {
        Create = TimeSpan.FromMinutes(20),
        Update = TimeSpan.FromMinutes(20),
    },
};
```

See Pulumi [`customTimeouts`](https://www.pulumi.com/docs/iac/concepts/resources/options/customtimeouts/).
`requestTimeoutSeconds` and `OSANO_API_TIMEOUT_SECONDS` govern individual HTTP
calls, not the whole asynchronous publication wait.

Cancellation and timeout stop provider polling promptly, but an operation
already accepted by Osano may continue. Check its status before retrying. A
later `pulumi up` can safely join an in-progress operation through `409`; do not
launch repeated immediate attempts.

### Persistent `outdated` status

Refresh is read-only: `pulumi refresh` can expose `outdated` but never publishes.
Confirm the program models all intended configuration and rule values, and that
its deterministic `changeToken` includes every publish-relevant desired value.
Because [`dependsOn`](https://www.pulumi.com/docs/iac/concepts/resources/options/dependson/)
controls ordering rather than update triggering, changing rules without changing
the token cannot republish. After resolving external drift, deliberately change
the token (for example through a stable desired publication revision) and apply
once. Do not continually vary the token to force immediate retries.

### Delayed CDN propagation after `published`

The provider completes when the Customer REST API proves the accepted operation
is `published`. Osano's CDN may take up to 15 minutes after that point to serve
the newest script revision at every edge. Keep the exact returned tag first in
the site `<head>` with no `async` or `defer`, and allow the propagation window
before diagnosing the script as stale. See Osano's
[direct Customer REST API `publishConfig` operation](https://developers.osano.com/customer-rest-api#tag/cmp/operation/publishConfig)
and [Consent JavaScript API](https://developers.osano.com/cmp/javascript-api/developer-documentation-consent-javascript-api).

## Cookie Consent create failed with a server error

Config and rule creates are not retried after `500`, `502`, or `504`, because
Osano may already have created the resource before the error was returned.
Osano has no delete endpoint for configurations, so a blind retry could leave a
permanent duplicate. `429` and `503` are still retried because they mean the
request was not processed.

Before re-running `pulumi up`, check Osano for a configuration or rule matching
your inputs. If one exists, import it instead of creating another:

```bash
pulumi import osano:index:CookieConsentConfig consentConfig <configId>
pulumi import osano:index:CookieConsentRule analyticsRule <configId>/<ruleId>
```

## Other API errors

### `400 Bad Request`

Check the Pulumi diagnostic for the operation and Osano validation response.
Fix invalid IDs, configuration values, or missing fields, then rerun preview.

### `401 Unauthorized`

Verify the route uses the correct key type, the key has not expired, and it
matches the production/sandbox environment. Rotate it with encrypted Pulumi
config and apply once.

## Debug strategy

1. Re-run with `--logtostderr --logflow -v=9 2> pulumi-debug.log` and redact
   credentials and PII (see [logging](logging.md)).
2. Use `pulumi refresh` to inspect remote status without mutation.
3. For Unified Consent, use `getUnifiedConsent` to inspect the subject's latest
   state.
4. If the error persists, open an issue with sanitized logs, the status code,
   the relevant resource ID, and the provider version (`pulumi plugin ls` lists
   the installed `osano` plugin versions).

Never share API keys or real subject identifiers in public threads.
