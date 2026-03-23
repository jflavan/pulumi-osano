# Release Guide

This document expands on the checklist with concrete commands for cutting a release of the Osano Pulumi provider.

## 1. Create a release branch

```bash
git checkout -b release/vX.Y.Z
```

## 2. Update versions

- Set `PROVIDER_VERSION` in your shell (example: `export PROVIDER_VERSION=1.1.0`).
- Run `make codegen` to regenerate the schema and SDKs with that version baked in.
- Update `README.md`, `docs/UPGRADE.md`, and other docs with noteworthy changes.

## 3. Run validation targets

```bash
mise exec -- gofmt -w $(find provider tests -name '*.go')
make lint
make test_provider
make build_sdks
```

If any step fails, fix the issue, rerun the command, and amend the branch.

## 4. Draft the release PR

- Summarize the key changes.
- Include links to the Osano API docs that motivated the work.
- Tag maintainers for review.

## 5. Tag and publish

After the PR merges into `main`:

```bash
git checkout main
git pull
git tag vX.Y.Z
git push origin vX.Y.Z
```

## 6. Publish SDKs

Publishing is automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml) once the tag is pushed. The workflow:

- builds provider binaries with GoReleaser,
- publishes Node.js, Python, and .NET SDKs,
- publishes the Java SDK to Maven Central,
- and publishes the Go SDK through Pulumi's Go SDK action.

Before tagging, make sure the required trusted-publishing and registry credentials are configured in GitHub Actions:

- npm Trusted Publisher for `@jflavan/pulumi-osano`
- PyPI Trusted Publisher for `pulumi-osano`
- NuGet trusted publishing / `NUGET_USERNAME`
- Maven Central publishing credentials and signing secrets

## 7. Announce

Post the release summary in GitHub Discussions and link to any new examples.

Follow these steps for every release so users can rely on predictable artifacts.
