# Performance Notes

The provider is lightweight: it simply serializes the Pulumi inputs and calls the Osano REST API. Still, there are a few considerations when running it at scale.

## Throughput

- Osano enforces rate limits per API key. Batch multiple consent actions into a single Pulumi resource when possible to minimize calls.
- Separate workloads into distinct stacks (for example, `consents-eu`, `consents-us`) to avoid throttling large previews.

## Latency

- Default HTTP timeout is 60 seconds. Override with `osano:requestTimeoutSeconds` or `OSANO_API_TIMEOUT_SECONDS` for slower integration tests.
- The provider serializes requests; there is no built-in parallelism. Use Pulumi's `pulumi.all` / `apply` features for client-side concurrency if needed.

## State Size

- Each `osano:index:Consent` resource stores the serialized input and latest response. Keep attributes compact to avoid bloating state snapshots.
- Use stack outputs sparingly—export aggregated metrics instead of giant response payloads.

## Preview Optimization

During `pulumi preview` the provider skips outbound HTTP calls and returns the predicted output immediately. This keeps previews fast even when creating dozens of resources.
