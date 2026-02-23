# pulumi-osano

Pulumi provider for managing [Osano](https://www.osano.com/) consent management resources via the Osano Customer REST API and Unified Consent Core API.

## Resources

| Resource | API | Delete support |
|---|---|---|
| `osano:CookieConsentConfig` | `/v1/cookie-consent/configs` | No (orphans on destroy) |
| `osano:CookieConsentRule` | `/v1/cookie-consent/rules` | Yes |

## Provider configuration

| Key | Description | Required |
|---|---|---|
| `osano:osanoApiKey` | Customer REST API key (`x-osano-api-key`). Secret. | For config/rule resources |
| `osano:ucApiKey` | Unified Consent Core API key (`x-uc-api-key`). Secret. | For UC Core resources |
| `osano:customerBaseUrl` | Override Customer REST API base URL (default `https://api.osano.com`). | No |
| `osano:ucBaseUrl` | Override UC Core API base URL (default `https://uc.api.osano.com`). | No |

## Docs
- Pulumi provider guide: https://www.pulumi.com/docs/iac/guides/building-extending/providers/build-a-provider/
- Osano UC Core OpenAPI: https://developers.osano.com/uc/core-api/openapi

## Plan
See [PLAN.md](./PLAN.md).

## Prerequisites
- Go 1.25+
- Pulumi CLI (for schema/SDK generation)

## Build

```sh
make provider      # build binary to bin/
make test           # run all tests with race detector
make schema         # generate schema.json (requires Pulumi CLI)
make sdk/nodejs     # generate Node.js SDK (requires schema)
```

Or without Make:

```sh
cd provider && go build ./cmd/pulumi-resource-osano
cd provider && go test -race -v -count=1 ./...
```

## Example

See [`examples/cookie-consent-config-ts/`](./examples/cookie-consent-config-ts/) for a TypeScript example that creates a config and rule.

## Notes
- The HTTP client retries 429 and 5xx responses with exponential backoff (respects `Retry-After` header).
- Osano does not expose a delete endpoint for CMP configs — `pulumi destroy` will orphan the remote config.
- CMP rules have full CRUD support including delete.
- See `PLAN.md` for current scope and next steps.
