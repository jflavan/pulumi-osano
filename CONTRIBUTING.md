# Contributing to pulumi-osano

## Prerequisites

- Go 1.25+
- Pulumi CLI (for schema/SDK generation)

## Development

```sh
# Build the provider binary
make provider

# Run tests
make test

# Generate schema (requires Pulumi CLI)
make schema

# Generate an SDK (e.g. nodejs)
make sdk/nodejs
```

## Adding a new resource

1. Create a new file in `provider/` (e.g. `my_resource.go`).
2. Implement the resource using `pulumi-go-provider/infer` interfaces (`CustomCheck`, `CustomDiff`, `CustomRead`, `CustomUpdate`, `CustomDelete`).
3. Register the resource in `provider/provider.go` via `WithResources(...)`.
4. Add unit tests in `provider/my_resource_test.go`.
5. Rebuild the provider and regenerate the schema + SDKs.

## Running tests

```sh
make test
```

Tests use the standard Go testing framework. Integration tests (if present) are skipped unless the `OSANO_API_KEY` environment variable is set.

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Resource IDs in URL paths must use `url.PathEscape()`.
- Keep test coverage for Check and Diff logic on every resource.
