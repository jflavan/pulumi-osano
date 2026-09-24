# Contributing to pulumi-osano

Thanks for helping improve the Pulumi Osano provider! The project mirrors the Pulumi native-provider boilerplate, so small changes can have broad effects. Please read this guide before opening a pull request.

## Quick Start

1. **Fork and clone** the repository.
2. **Install toolchains** via [mise](https://mise.jdx.dev/) and activate them so `make` uses the pinned Go, Node, and Pulumi:
   ```bash
   eval "$(mise activate zsh)"   # or bash
   mise install
   ```
3. **Export credentials** when running examples or manual tests:
   ```bash
   export OSANO_API_KEY="customer-rest-cmp-key"
   export OSANO_UC_API_KEY="unified-consent-key"
   ```
4. **Make your changes** (see workflow below).
5. **Run provider tests**:
   ```bash
   make test_provider
   ```
6. **Submit a pull request** with context and test results.

To run an example against Osano from your clone, build and install the local provider plugin first; see section 1 of the [end-to-end workflow guide](docs/end-to-end-workflow.md).

## Development Workflow

### Prerequisites

- Go (managed by mise, currently Go 1.24)
- Node.js 24.x (mise currently pins 24.13.0)
- Python 3.11
- .NET 8.0
- Java 11+
- Pulumi CLI + pulumictl (installed by `mise install`)

> Tip: `eval "$(mise activate zsh)"` (or bash) before running make targets so the managed toolchain is on your `PATH`.

### Editing Provider Code

1. Modify files under `provider/`.
2. Run formatting and tidy tools if necessary (`gofmt`, `go mod tidy`).
3. **Run `make codegen`** before committing. This regenerates the provider schema plus every SDK. CI fails if generated code is stale.
4. Stage the Go changes **and** all generated SDK updates together.

### Adding or Updating Resources / Functions

1. Implement the resource/invoke in a dedicated `provider/<name>.go` file (for example `consent_resource.go` or `cookie_consent_rule.go`) or in `provider/functions.go` for invokes, using the existing implementations as a reference.
2. Add or update unit tests (mock HTTP recommended).
3. Run `make codegen`.
4. Add/refresh examples (see [EXAMPLES.md](EXAMPLES.md)). At minimum provide a TypeScript example and README; multi-language samples are encouraged for widely used functionality.
5. Run validation:
   ```bash
   make lint
   make test_provider
   make build_examples
   make test_e2e_compile
   ```
6. Document behavior changes in `docs/` and/or `README.md` as appropriate.

### Important Make Targets

| Command | Purpose |
| --- | --- |
| `make codegen` | Regenerate schema + SDKs (required after provider edits) |
| `make provider` | Build only the provider binary |
| `make build` | Build provider **and** SDKs |
| `make test_provider` | Run Go unit tests (mocked HTTP, no tokens needed) |
| `make lint` | Run golangci-lint with repo defaults (uses `--fix`, so review the rewritten files) |
| `make build_cookie_consent_examples` | Compile the canonical C# and companion TypeScript CMP examples without contacting Osano |
| `make build_examples` | Compile the Cookie Consent examples and the Go quickstart |
| `make test_e2e_compile` | Vet every `tests/e2e` build-tag set without credentials |
| `make test_scripts` | Run the Python tests for the SDK post-processing scripts |

### Commit Message Guidance

Use [Conventional Commits](https://www.conventionalcommits.org/). Examples:

- `feat(consent): add subject status output`
- `fix(config): honor OSANO_API_TIMEOUT_SECONDS`
- `docs(examples): expand quickstart README`
- `test(consent): cover unified consent errors`

Add `BREAKING CHANGE:` in the footer for breaking API or behavior updates.

### Pull Request Checklist

- [ ] `mise install` run at least once locally
- [ ] `make codegen` run after provider edits
- [ ] Tests (`make test_provider`) pass locally
- [ ] Docs/examples updated when behavior changes
- [ ] PR description covers *what*, *why*, and *how tested*
- [ ] Linked issues (if any) referenced in the PR body
- [ ] `CodeQL gate` and `Code scanning results / CodeQL` pass (both are required to merge)

### Code Scanning (CodeQL)

`.github/workflows/codeql.yml` (**CodeQL Advanced**) runs CodeQL with the `security-extended` queries on every pull request to `main`, every push to `main`, weekly, and on demand from the Actions tab. It needs no secrets, so fork pull requests get the same checks once a maintainer approves a first-time contributor's run. It analyzes the GitHub Actions workflows and composite actions, the non-test Go code in all three Go modules, the Python scripts, example, and SDK, and the TypeScript examples and Node.js SDK. The generated C# and Java SDKs, the C# example, and Go `_test.go` files (including the build-tagged e2e tests) are not scanned.

- Pull requests are analyzed in full, not only on the lines they change, so an alert that a pull request causes elsewhere (for example by deleting a guard or a `permissions:` block) is found too.
- On a pull request, `Analyze (<language>)` fails when the pull request introduces a CodeQL alert of any severity: an open alert with no counterpart, by rule, file, and message, that is open or dismissed on `main`. Each counterpart matches one alert, so a second copy of an existing problem still fails. The job annotates each new alert and lists them in the job summary. Until `main` has an analysis for a language, only alerts on lines the pull request changes count. The job also fails when the analysis itself fails, for example during tool setup or upload, or when no Go module can be extracted. Pushes to `main` and scheduled runs never fail because of alerts; their alerts go to the Security and quality tab.
- If the job cannot read the code scanning API, as can happen with a fork's read-only token, it instead fails on any CodeQL result on the lines the pull request changes. That fallback cannot see dismissals or alerts elsewhere.
- `CodeQL gate` fails when any `Analyze` job fails or is cancelled. A branch ruleset on `main` requires `CodeQL gate` and `Code scanning results / CodeQL` and requires the pull request branch to be up to date with `main`, so a pull request with a new CodeQL alert cannot be merged. After another pull request merges, update the branch and let CodeQL run again. The ruleset has no bypass and also blocks direct pushes of commits that have not passed these checks.
- Only `pull_request` runs report the required `CodeQL gate` name. Running **CodeQL Advanced** by hand reports `CodeQL gate (workflow_dispatch)`, which never counts for a pull request; to retry, use **Re-run failed jobs** on the pull request's own run. If a required check shows "Expected — Waiting for status to be reported", for example after a pull request's base is changed to `main`, push a commit or close and reopen the pull request. Do not put `[skip ci]` in a pull request's head commit.
- These checks run the pull request's own copy of `codeql.yml`, `.github/scripts/`, the Makefile, and the Go toolchain files, so they cannot police a pull request that changes those files. Review such changes by hand before approving a run or merging.
- The Go analysis uses autobuild, which runs `make`. The first Makefile target, `ensure`, runs `go mod tidy`, so keep a cheap target first. The job sets `GOTOOLCHAIN=local` with the root `go.mod` toolchain, so a nested `go.mod` that needs a newer Go is skipped: `Analyze (go)` stays green and only a warning appears under **Security and quality > Code scanning > Tool status**. Check that page after changing a `go.mod`.
- `Code scanning results / CodeQL` fails when the pull request adds, on lines it changes, an alert of `high` or `critical` security severity, or an `error` alert that has no security severity. Security severity takes precedence, so a Medium or Low security alert does not fail the check even when its level is `error`. It still appears as an annotation.
- To dismiss a false positive, go to **Security and quality > Code scanning**, dismiss the alert with a reason, then use **Re-run failed jobs** on the pull request's CodeQL run. Inline suppression comments (`codeql[...]`, `lgtm`) have no effect in this setup for any language. For an alert in `sdk/`, fix the provider or schema and run `make codegen`. Never hand-edit generated files.
- Keep code scanning "default setup" disabled in the repository settings. Do not rename the workflow file, the `analyze` job, the `CodeQL gate` job, the matrix languages, or the `/language:<language>` categories. Renaming `CodeQL gate` also requires updating the ruleset, or every pull request waits for a check that never reports. If one is renamed, run the workflow on `main` (**Actions > CodeQL Advanced > Run workflow**) and delete the old configuration under **Security and quality > Code scanning > Tool status**. Until then, pull requests show a neutral "configuration not found" result.
- Actions are pinned to commit SHAs. To upgrade CodeQL, move `github/codeql-action/init` and `github/codeql-action/analyze` to the same new release commit in one change, and update the `# vX.Y.Z` comments.

## Examples

- Every user-facing resource or invoke needs at least one runnable TypeScript example plus a README explaining inputs/outputs.
- Add Python/Go/.NET/Java variants when feasible, especially for high-traffic APIs.
- See [examples/README.md](examples/README.md) and the detailed guidance in [EXAMPLES.md](EXAMPLES.md).

## Project Structure

```
provider/           Go provider implementation (hand-written)
provider/cmd/       Provider binary + embedded schema
sdk/                Generated language SDKs (never edit manually)
examples/           Pulumi programs that double as docs
docs/               Conceptual guides and release notes
```

## Tooling Notes

- Follow the patterns documented in `.github/copilot-instructions.md` when using GitHub Copilot / GPT-based agents.
- Run `mise doctor` if you encounter CLI version issues.
- Pulumi home is pinned to `.pulumi`; do not delete unless you know the impact.

## Getting Help

- **Issues:** use https://github.com/jflavan/pulumi-osano/issues
- **Discussions:** use https://github.com/jflavan/pulumi-osano/discussions for design/usage questions
- **Docs:** start with [README.md](README.md) and the content under `docs/`

## Code of Conduct

Participation is governed by our [Code of Conduct](CODE-OF-CONDUCT.md). Report unacceptable behavior to the maintainers as described there.

## License

Contributions are accepted under the project license (see [LICENSE](LICENSE)). By submitting a PR you confirm you have the right to license your work similarly.

---

**Thank you for contributing to pulumi-osano!** 🎉
