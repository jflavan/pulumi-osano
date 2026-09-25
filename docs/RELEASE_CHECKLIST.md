# Release Checklist

Use this checklist whenever publishing a new `pulumi-osano` provider release.
[RELEASE_GUIDE.md](RELEASE_GUIDE.md) explains each step in detail.

1. **Check the one-time setup** (first release, or after rotating credentials)
   - Repository secrets exist: `NUGET_USERNAME`, `MAVEN_CENTRAL_USERNAME`,
     `MAVEN_CENTRAL_PASSWORD`, `JAVA_SIGNING_KEY`, `JAVA_SIGNING_KEY_ID`, and
     `JAVA_SIGNING_PASSWORD`. `NPM_TOKEN` is optional (see the npm bootstrap in the release
     guide).
   - Trusted publishers point at owner `jflavan`, repository `pulumi-osano`, workflow
     `release.yml`, and no environment: the PyPI pending publisher for `pulumi-osano` and the
     NuGet trusted publishing policy (and, from the second release on, the npm trusted publisher
     for `@jflavan/pulumi-osano`).
   - The Maven Central namespace `io.github.jflavan` is verified and the signing public key is on
     a public key server.
2. **Do not bump versions in files**
   - The release version comes from the tag. The committed schema and SDKs stay at the Makefile's
     development version (`0.1.0-alpha.0+dev`); the release workflow stamps the real version.
3. **Run the quality gates on `main`**

   ```bash
   make codegen
   make test_provider
   make build_sdks
   make build_examples
   make test_e2e_compile
   git diff --exit-code
   ```

   - Also run `make provider` so the version-stamped Go provider compiles and
     `make lint` to execute `golangci-lint` with repository defaults.
   - The final diff check proves schema, SDKs, and copied package READMEs are current.
   - Confirm the latest `CodeQL Advanced` run on `main` succeeded and that
     **Security and quality > Code scanning** shows no open critical or high alerts.
   - Check that `github/codeql-action` in `.github/workflows/codeql.yml` is on the
     latest v4 release. If it is not, bump `init` and `analyze` to the same commit
     SHA in one change, and update the `# vX.Y.Z` comments.
4. **Run a release dry run**
   - `gh workflow run release.yml --ref main` (or **Actions > release > Run workflow** on `main`).
     Manual runs never publish.
   - Every job must pass: the provider and SDK builds, `publish (dry run)` (GoReleaser snapshot
     and archive names), `publish_sdks (dry run)` (`npm publish --dry-run`, `twine check`, the
     `.nupkg`, the Go SDK tag), and `publish_java_sdk (dry run)` (`publishToMavenLocal`).
5. **Validate documentation**
   - README quickstart instructions must reflect the published install paths.
   - Repo-local examples must clearly document any required local SDK build steps.
   - Cookie Consent docs must agree that preview/refresh/import do not publish,
     rules delete upstream, and configuration/publication deletes are state-only.
   - The Pulumi Registry pages `docs/_index.md` and `docs/installation-configuration.md` match
     the provider's resources, functions, and configuration, start with YAML front matter, and
     use only absolute links.
6. **Update the changelog**
   - In the release PR, move the `## [Unreleased]` entries of `CHANGELOG.md` into
     `## [X.Y.Z] - YYYY-MM-DD` and add its compare link. The changelog never carries a guessed
     date: `## [0.1.0] - TBD` stays `TBD` until the release PR, where `TBD` becomes the date you
     tag (UTC).
   - Summarize user-facing changes, new resources, and breaking updates.
   - Highlight Osano API version changes and any new required scopes.
   - For Cookie Consent changes, call out `scriptSrc`, `scriptTag`, the
     caller-managed `changeToken`, default preservation of unclassified
     discoveries, composite rule imports, and configurations/publications
     retained upstream after destroy.
7. **Tag and publish**
   - After the release PR merges: `git checkout main && git pull && git tag vX.Y.Z && git push origin vX.Y.Z`.
   - Verify the GitHub Actions `release` workflow completes successfully for provider binaries and every SDK.
   - If a job fails, fix the cause and use **Re-run failed jobs**; publishing steps skip versions
     that are already published. If the `publish` (GoReleaser) job failed after creating the
     GitHub release, delete the release (keep the tag) first. Never move a published tag.
   - Confirm npm, PyPI, NuGet, Maven Central, and Go publication all completed before announcing the release.
8. **Post-release follow-up**
   - Check that the plugin installs:
     `pulumi plugin install resource osano X.Y.Z --server github://api.github.com/jflavan/pulumi-osano`.
   - First release of the npm package only: when the npm step fails for lack of a trusted
     publisher, publish the CI-built `nodejs-sdk.tar.gz` by hand, add the npm trusted publisher
     (allow **npm publish**, not only `npm stage publish`), and re-run the failed jobs. See
     "npm: bootstrap, then trusted publishing" in the release guide.
   - After the first release only: list the package in the Pulumi Registry. Open a PR to
     [pulumi/registry](https://github.com/pulumi/registry) that adds
     `{"repoSlug": "jflavan/pulumi-osano", "schemaFile": "provider/cmd/pulumi-resource-osano/schema.json"}`
     to `community-packages/package-list.json` and `"John Flavan": "john_flavan"` to
     `tools/resourcedocsgen/pkg/publishers/publisher-names.json`, then fix anything the PR's
     automated fact sheet flags and comment `/check`. Later releases are picked up by the
     registry automatically within a day; confirm the new version at
     `https://www.pulumi.com/registry/packages/osano/`.
   - Monitor issues for regressions and update the roadmap if new Osano endpoints were unlocked.

Keep the checklist updated as automation improves.
