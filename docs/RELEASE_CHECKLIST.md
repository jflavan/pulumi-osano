# Release Checklist

Use this checklist whenever publishing a new `pulumi-osano` provider release.

1. **Update the changelog**
   - Summarize user-facing changes, new resources, and breaking updates.
   - Highlight Osano API version changes and any new required scopes.
   - For Cookie Consent changes, call out `scriptSrc`, `scriptTag`, the
     caller-managed `changeToken`, default preservation of unclassified
     discoveries, composite rule imports, and configurations/publications
     retained upstream after destroy.
2. **Bump the provider version**
   - Set the new semver in the release PR (environment variable `PROVIDER_VERSION`).
   - Regenerate schema + SDKs with `make codegen` and commit the results.
3. **Run the quality gates**

   ```bash
   make codegen
   make test_provider
   make build_sdks
   make build_cookie_consent_examples
   git diff --exit-code
   ```

   - Also run `make provider` so the version-stamped Go provider compiles and
     `make lint` to execute `golangci-lint` with repository defaults.
   - The final diff check proves schema, SDKs, and copied package READMEs are current.
4. **Validate documentation**
   - README quickstart instructions must reflect the published install paths.
   - Repo-local examples must clearly document any required local SDK build steps.
   - Cookie Consent docs must agree that preview/refresh/import do not publish,
     rules delete upstream, and configuration/publication deletes are state-only.
5. **Publish artifacts**
   - Tag the repo: `git tag vX.Y.Z && git push origin vX.Y.Z`.
   - Verify the GitHub Actions `release` workflow completes successfully for provider binaries and every SDK.
   - Confirm npm, PyPI, NuGet, Maven Central, and Go publication all completed before announcing the release.
6. **Post-release follow-up**
   - Verify that the Pulumi registry entry shows the correct version.
   - Monitor issues for regressions and update the roadmap if new Osano endpoints were unlocked.

Keep the checklist updated as automation improves.
