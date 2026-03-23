# Release Checklist

Use this checklist whenever publishing a new `pulumi-osano` provider release.

1. **Update the changelog**
   - Summarize user-facing changes, new resources, and breaking updates.
   - Highlight Osano API version changes and any new required scopes.
2. **Bump the provider version**
   - Set the new semver in the release PR (environment variable `PROVIDER_VERSION`).
   - Regenerate schema + SDKs with `make codegen` and commit the results.
3. **Run the quality gates**
   - `make provider` – ensures the Go provider compiles with the new version stamp.
   - `make test_provider` – runs the Go provider test suite.
   - `make lint` – executes `golangci-lint` over the provider.
   - `make build_sdks` – compiles all language SDKs to ensure the schema stays valid.
4. **Validate documentation**
   - README quickstart instructions must reflect the published install paths.
   - Repo-local examples must clearly document any required local SDK build steps.
5. **Publish artifacts**
   - Tag the repo: `git tag vX.Y.Z && git push origin vX.Y.Z`.
   - Verify the GitHub Actions `release` workflow completes successfully for provider binaries and every SDK.
   - Confirm npm, PyPI, NuGet, Maven Central, and Go publication all completed before announcing the release.
6. **Post-release follow-up**
   - Verify that the Pulumi registry entry shows the correct version.
   - Monitor issues for regressions and update the roadmap if new Osano endpoints were unlocked.

Keep the checklist updated as automation improves.
