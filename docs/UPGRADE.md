# Upgrade Guide

This guide captures breaking changes and migration tips between provider versions.

## 0.x → 0.y

The provider is still pre-1.0, so we may ship breaking changes between minor releases. Always pin your Pulumi program to an explicit version in `Pulumi.yaml`:

```yaml
plugins:
  providers:
    - name: osano
      version: 0.2.0
```

### Consent resource shape changes

- Fields may be renamed as Osano expands the API. Review the release notes for each version and update your Pulumi code accordingly.
- When new required fields are added, run `pulumi preview` to spot the diff before applying.

### SDK namespace changes

If you consume the generated SDKs directly:

- Node.js: `import * as osano from "@jflavan/pulumi-osano";`
- Python: `import pulumi_osano as osano`
- Go: `github.com/jflavan/pulumi-osano/sdk/go/osano`

Check your lockfiles to ensure the new version is installed.

### Rolling out upgrades safely

1. Run `make codegen && make build_sdks` locally to verify that generation still works.
2. Update your example projects and run `pulumi preview`.
3. Deploy to a staging stack before touching production.

Report regressions as GitHub issues with stack traces and the Osano API response if available.
