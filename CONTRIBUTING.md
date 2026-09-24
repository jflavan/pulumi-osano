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
- [ ] `Code scanning results / CodeQL` passes, or any new alert is fixed or explained in the PR

### Code Scanning (CodeQL)

`.github/workflows/codeql.yml` (**CodeQL Advanced**) runs CodeQL with the `security-extended` queries on every pull request to `main`, every push to `main`, weekly, and on demand from the Actions tab. It needs no secrets, so fork pull requests get the same checks once a maintainer approves a first-time contributor's run. It analyzes the GitHub Actions workflows and composite actions, the Go code (all three Go modules), the Python scripts, example, and SDK, and the TypeScript examples and Node.js SDK. The generated C# and Java SDKs and the C# example are not scanned.

- `Analyze (<language>)` fails only when the analysis itself fails, for example during tool setup, Go extraction, or upload. Read the job log. The Go analysis uses autobuild, which runs `make`. The first Makefile target, `ensure`, runs `go mod tidy`, so keep a cheap target first.
- `Code scanning results / CodeQL` fails when the pull request adds an alert of `error` severity, or of `high` or `critical` security severity, on lines it changes. Medium and low alerts appear as annotations but do not fail the check.
- To dismiss a false positive, go to **Security > Code scanning** and give a reason. Workflow alerts cannot be suppressed with inline comments. For an alert in `sdk/`, fix the provider or schema and run `make codegen`. Never hand-edit generated files.
- Keep code scanning "default setup" disabled in the repository settings. Do not rename the workflow file, the `analyze` job, the matrix languages, or the `/language:<language>` categories.
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
