# Release Guide

This guide explains how a release of the Osano Pulumi provider works and gives the commands for each
step. [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md) is the short list to follow while releasing.

## How a release works

Pushing a `vX.Y.Z` tag runs [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which:

1. builds and tests the provider and regenerates the schema and every SDK (`prerequisites`,
   `build_sdks`);
2. builds the provider binaries with GoReleaser, creates the GitHub release with the archives,
   checksums, and SBOMs, and attests their build provenance (`publish`);
3. publishes the Node.js SDK to npm, the Python SDK to PyPI, and the .NET SDK to NuGet
   (`publish_sdks`), and the Java SDK to Maven Central (`publish_java_sdk`);
4. pushes the `sdk/go/osano/vX.Y.Z` tag for the Go SDK (`publish_go_sdk`).

The version always comes from the tag. The committed schema and SDKs keep the development version
from the Makefile (`PROVIDER_VERSION ?= 0.1.0-alpha.0+dev`), and the workflow stamps the real version
at build time, so a release needs no version bump in any file. Do not commit SDKs regenerated with
a release version: CI regenerates them with its own version and only tolerates differences in the
version-stamped files.

## One-time setup

### Repository secrets

Add these under **Settings > Secrets and variables > Actions > Repository secrets**. `GITHUB_TOKEN`
is provided automatically.

| Secret | Used by | Value |
| --- | --- | --- |
| `NPM_TOKEN` | `publish_sdks` (npm), **optional** | Not needed once the npm trusted publisher exists. Only an alternative way to bootstrap a brand-new npm package; see [npm: bootstrap, then trusted publishing](#npm-bootstrap-then-trusted-publishing). |
| `NUGET_USERNAME` | `publish_sdks` (NuGet login) | The nuget.org account name (profile name, not the email address) that owns the trusted publishing policy. |
| `MAVEN_CENTRAL_USERNAME` | `publish_java_sdk` | The username half of a Maven Central Portal user token. |
| `MAVEN_CENTRAL_PASSWORD` | `publish_java_sdk` | The password half of the same user token. |
| `JAVA_SIGNING_KEY` | `publish_java_sdk` | The ASCII-armored GPG private key that signs the Maven artifacts. |
| `JAVA_SIGNING_KEY_ID` | `publish_java_sdk` | The ID of that key: its last 8 hexadecimal characters, the form Gradle's signing plugin expects. |
| `JAVA_SIGNING_PASSWORD` | `publish_java_sdk` | The passphrase of that key. |

The Maven Central public key must be published to a public key server (for example
`keys.openpgp.org`) so Maven Central can verify the signatures.

### Trusted publishers

Every trusted publisher points at this repository and workflow. The values are case-sensitive and
must match exactly; leave the environment empty, because `release.yml` does not use a GitHub
environment.

| Registry | Where | Settings |
| --- | --- | --- |
| npm | npmjs.com > `@jflavan/pulumi-osano` > Settings > Trusted Publisher (after the first publish) | GitHub Actions; Organization or user `jflavan`; Repository `pulumi-osano`; Workflow filename `release.yml`; Environment empty |
| PyPI | pypi.org > Your account > Publishing > Add a new pending publisher | PyPI project name `pulumi-osano`; Owner `jflavan`; Repository name `pulumi-osano`; Workflow name `release.yml`; Environment name empty |
| NuGet | nuget.org > Trusted Publishing > Create policy | Repository owner `jflavan`; Repository `pulumi-osano`; Workflow file `release.yml`; Environment empty. The `NUGET_USERNAME` secret names the policy owner. |

Maven Central does not support trusted publishing. It needs a verified `io.github.jflavan`
namespace on central.sonatype.com (the Java SDK's group `io.github.jflavan.pulumi` is inside it), the
user token, and the signing key above.

### npm: bootstrap, then trusted publishing

npm cannot configure a trusted publisher for a package that does not exist yet, so the very first
version of `@jflavan/pulumi-osano` has to be published some other way; every later release uses
trusted publishing. npm 11.5+ first tries to exchange the job's GitHub OIDC token, and only falls back
to `NODE_AUTH_TOKEN` (the optional `NPM_TOKEN` secret, read through the `.npmrc` that
`actions/setup-node` writes) when that exchange fails. Once a trusted publisher exists, its token
replaces the `.npmrc` token, so an empty, missing, or leftover `NPM_TOKEN` cannot break trusted
publishing.

`v0.1.0` was bootstrapped without any token, by publishing the CI-built package by hand. npm is
restricting tokens that bypass 2FA for direct publishing, so prefer this over an `NPM_TOKEN`:

1. Push the `vX.Y.Z` tag. With no trusted publisher and no `NPM_TOKEN`, the npm step of
   `publish_sdks` fails with an authentication error before PyPI and NuGet run, and
   `publish_go_sdk` is skipped. The GitHub release and SDK artifacts are already built.
2. Download the Node.js SDK the run built and publish that exact package from a maintainer machine:

   ```bash
   gh run download <run-id> -R jflavan/pulumi-osano -n nodejs-sdk.tar.gz -D /tmp/npm-bootstrap
   mkdir /tmp/npm-bootstrap/sdk && tar -zxf /tmp/npm-bootstrap/nodejs.tar.gz -C /tmp/npm-bootstrap/sdk
   npm login --auth-type=web          # signs in through the browser, including 2FA
   cd /tmp/npm-bootstrap/sdk/bin && npm publish --access public
   ```

   This first version has no provenance statement; later versions published by the workflow do.
3. On npmjs.com, open the package's **Settings > Trusted publishing** and add the trusted publisher
   from the table above. Trusted publishers created after 3 September 2026 allow only
   `npm stage publish` by default: also allow **npm publish**, because the workflow publishes
   directly.
4. Re-run the failed jobs of the release run. The npm step skips the version that is now on npm,
   PyPI and NuGet publish, and `publish_go_sdk` pushes the Go SDK tag.
5. Optionally, under **Publishing access**, select **Require two-factor authentication and disallow
   tokens**. Trusted publishing keeps working because it does not use a token.

If you bootstrap with an `NPM_TOKEN` instead, delete the secret and revoke the token once the
trusted publisher exists. If a later release fails with an authentication error, check the trusted
publisher values first: npm reports a failed OIDC exchange only in verbose logs and then falls back
to the (absent) token.

## Dry run

A manual run of the release workflow is always a dry run. There is no input that makes it publish:
every publishing job runs only for a tag push, and the dry-run jobs have read-only permissions and no
registry credentials.

Start one from **Actions > release > Run workflow**, or with the GitHub CLI:

```bash
gh workflow run release.yml --ref main
gh run watch
```

Run it from a branch (`main` or a release branch), not a tag: the version step does not support a
manual run on a tag. The run uses the version `pulumi/provider-version-action` computes for a
branch: the next minor version after the latest GitHub release as an alpha, for example
`0.1.0-alpha.1727200000` from `main` before the first release (from other branches the short commit
hash is appended, for example `0.1.0-alpha.1727200000+abc1234`).

A dry run:

- builds and tests the provider and regenerates and builds every SDK exactly like a release,
  including the worktree-clean checks;
- runs GoReleaser with `--snapshot`, which builds every release archive, checksum, and SBOM without
  creating a GitHub release, checks the archive names that `pulumi plugin install` downloads, and
  uploads them as the `dry-run-provider-archives` artifact (kept for 7 days);
- runs `npm publish --dry-run` for the Node.js SDK with the same script, npm version, and dist-tag
  logic as a release;
- runs `twine check` on the Python distributions, lists the `.nupkg`, and builds the Maven
  publication into the runner's local Maven repository (`publishToMavenLocal`, unsigned);
- reports, for npm, PyPI, NuGet, Maven Central, and the Go SDK tag, whether that version is already
  published, which is what a re-run of a release would skip.

It never publishes to npm, PyPI, NuGet, or Maven Central, never pushes a tag, never creates a GitHub
release, and never attests provenance.

## Cut the release

1. Merge everything for the release into `main` and wait for `build` and `CodeQL Advanced` to pass.
2. Run a dry run from `main` and wait for it to pass.
3. In a release PR, move the `## [Unreleased]` entries of [CHANGELOG.md](../CHANGELOG.md) into a
   `## [X.Y.Z] - YYYY-MM-DD` section dated with the day you will tag, add the compare link at the
   bottom, and merge. For `v0.1.0` the section already exists as `## [0.1.0] - TBD`: replace `TBD`
   with the date.
4. Tag the merge commit on `main` and push the tag:

   ```bash
   git checkout main
   git pull
   git tag v0.1.0
   git push origin v0.1.0
   ```

5. Watch the run: `gh run watch` (or **Actions > release**).

Never move or re-push a tag that a release has published from. If a release needs a code change, fix
it on `main` and release the next patch version.

## Re-run a partially failed release

Use **Re-run failed jobs** on the failed run (or `gh run rerun <run-id> --failed`). It re-runs only
the failed jobs and the jobs that depend on them, and they reuse the artifacts the successful jobs
already uploaded. Every publishing step skips work that already happened:

| Job | On re-run |
| --- | --- |
| `publish_sdks`: npm | Skips when `@jflavan/pulumi-osano@X.Y.Z` is already on npm (`npm view`). |
| `publish_sdks`: PyPI | `skip-existing: true` skips files that are already uploaded. |
| `publish_sdks`: NuGet | `--skip-duplicate` skips a version that is already on nuget.org. |
| `publish_java_sdk` | Skips when `io/github/jflavan/pulumi/pulumi-osano/X.Y.Z/pulumi-osano-X.Y.Z.pom` exists on `repo1.maven.org`. |
| `publish_go_sdk` | `pulumi/publish-go-sdk-action` skips the commit and push when the `sdk/go/osano/vX.Y.Z` tag exists, and only refreshes the Go module cache. |

Two cases need care:

- **Maven Central lag.** A released version can take 30 minutes or more to appear on
  `repo1.maven.org`. Before re-running `publish_java_sdk` soon after a Java publish, check the
  deployment on the
  [Central Portal deployments page](https://central.sonatype.com/publishing/deployments). If it is
  published or still publishing, wait until the version appears on `repo1.maven.org` before
  re-running. Otherwise the job uploads a duplicate deployment, which Maven Central rejects, and the
  job fails without changing the published version.
- **The `publish` (GoReleaser) job failed.** GoReleaser does not replace assets that already exist
  on a release (`replace_existing_artifacts` is off). If it failed after creating the GitHub
  release, delete that release but keep the tag, for example `gh release delete v0.1.0 --yes`, then
  re-run the failed jobs. Do not use **Re-run all jobs** after `publish` succeeded: GoReleaser would
  fail on the existing release.

## After the release

1. Check the published packages: the GitHub release assets, npm, PyPI, NuGet, Maven Central, and
   `go list -m github.com/jflavan/pulumi-osano/sdk/go/osano@vX.Y.Z`.
2. Check that the plugin installs from the release:

   ```bash
   pulumi plugin install resource osano X.Y.Z --server github://api.github.com/jflavan/pulumi-osano
   ```

3. After the first release only, switch npm to trusted publishing (steps 4 to 6 of
   [npm: bootstrap, then trusted publishing](#npm-bootstrap-then-trusted-publishing)) and list the
   package in the Pulumi Registry (below).
4. Post the release summary in GitHub Discussions and link to any new examples.

### List the package in the Pulumi Registry

Once the provider is listed, the registry checks for new releases twice a day and publishes their
docs automatically, so this is needed once, after the first release. The process is described in
[Adding a new package](https://github.com/pulumi/registry/blob/master/docs/adding-a-new-package.md).
The registry reads everything from the release tag, so the following must be true at `v0.1.0`
(it is on `main` now):

- `provider/cmd/pulumi-resource-osano/schema.json` sets `publisher` (`John Flavan`), `logoUrl`
  (`assets/logo.png` on `main`), `displayName`, `pluginDownloadURL`, and `keywords` with
  `category/infrastructure` and `kind/native`. It has no `version`, so the registry takes the
  version from the tag.
- `docs/_index.md` (required) and `docs/installation-configuration.md` (optional) start with YAML
  front matter and use only absolute links.
- Both Osano API clients send the `pulumi-osano/<version>` user agent.
- `vX.Y.Z` is a published GitHub release, not a draft or prerelease, and the plugin installs with the
  command above.

Then:

1. Fork [pulumi/registry](https://github.com/pulumi/registry) and add one entry at the end of the
   `include` list in `community-packages/package-list.json`:

   ```json
   {
     "repoSlug": "jflavan/pulumi-osano",
     "schemaFile": "provider/cmd/pulumi-resource-osano/schema.json"
   }
   ```

2. In the same PR, add the new publisher to
   `tools/resourcedocsgen/pkg/publishers/publisher-names.json`. The key must equal the schema's
   `publisher`, and the value is the publisher's slug:

   ```json
   "John Flavan": "john_flavan"
   ```

3. Open the PR. Say that the provider is community maintained and give `@jflavan` as the contact.
   Automated checks post a fact sheet (docs generation, plugin install, npm, PyPI, and Go installs,
   publisher, doc lint). Fix anything flagged in this repository, release again if the fix must be
   in a release, and comment `/check` on the PR to re-run the checks. A Pulumi maintainer reviews
   and merges it.
4. After it merges, confirm the package appears at `https://www.pulumi.com/registry/packages/osano/`
   with the logo, the overview, and the Installation & Configuration page.

To change the registry's search terms, category, or logo later, change the provider's keywords,
publisher, or logo in `provider/provider.go`, regenerate the schema, and release. Do not edit the
registry's generated package files.
