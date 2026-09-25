# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, email the maintainer, John Flavan, at the address on the [@jflavan GitHub profile](https://github.com/jflavan).

### What to Include

When reporting a vulnerability, please include:

- A description of the vulnerability
- Steps to reproduce the issue
- Potential impact of the vulnerability
- Any suggested fixes (if applicable)

### Response Timeline

- **Initial Response**: Within 48 hours of receiving your report
- **Status Update**: Within 7 days with an assessment and remediation plan
- **Resolution**: Depending on complexity, typically within 30 days

### What to Expect

1. **Acknowledgment**: We will acknowledge receipt of your report within 48 hours.
2. **Assessment**: We will investigate and assess the severity of the issue.
3. **Communication**: We will keep you informed of our progress.
4. **Resolution**: Once fixed, we will release a patch and credit you (unless you prefer to remain anonymous).
5. **Disclosure**: We will coordinate with you on public disclosure timing.

## Security Best Practices for Users

### API Token Security

- **Never commit API tokens** to version control
- Use `pulumi config set osano:osanoApiKey <key> --secret` and `pulumi config set osano:unifiedConsentApiKey <key> --secret` to securely store tokens
- Alternatively, use the `OSANO_API_KEY` and `OSANO_UC_API_KEY` environment variables
- Rotate tokens regularly
- Use tokens with minimal required permissions

### Infrastructure Security

- Review Pulumi state files for sensitive data before sharing
- Use Pulumi's built-in encryption for secrets
- Consider using Pulumi Cloud or a secure backend for state storage

## Security Features

This provider implements several security measures:

- **TLS 1.2+**: API calls use HTTPS with Go's default TLS client configuration, which requires TLS 1.2 or higher
- **No Default Request-Body Logging**: Sensitive payloads are not logged by the provider by default
- **Input Validation**: Resource inputs are validated before API calls; invokes check that required inputs are present
- **SBOMs**: every provider archive on the GitHub release has a `.sbom.json` SBOM.
- **Build provenance (SLSA Build Level 2)**: every provider archive and SBOM on the GitHub release has a GitHub build provenance attestation (`gh attestation verify pulumi-resource-osano-vX.Y.Z-linux-amd64.tar.gz --owner jflavan`) and a SHA-256 checksum in `checksums.txt`.
- **Package provenance and signatures** (see [docs/PUBLISHING.md](docs/PUBLISHING.md) to verify each one):
  - PyPI: published with trusted publishing from `release.yml`, with PyPI publish attestations for the wheel and sdist.
  - npm: `0.1.0` was published by hand from the CI-built package and has no provenance statement. Later versions are published with trusted publishing from `release.yml`, with npm provenance. Registry signatures verify with `npm audit signatures`.
  - NuGet: published from `release.yml` with NuGet trusted publishing (a short-lived OIDC login, no stored API key).
  - Maven Central: every artifact is signed with the release signing key `5277 E261 0B7E 7021 6871  969A 4809 7CF9 4C3F 74F3`.
  - Go: module checksums are recorded in the Go checksum database (`sum.golang.org`).
- **Code Scanning**: GitHub CodeQL (`security-extended` queries) analyzes the Go provider, the generated Go, Node.js, and Python SDKs, the Python and TypeScript code, and the GitHub Actions workflows on every pull request to `main`, every push to `main`, and weekly. Pull requests that introduce a new CodeQL alert cannot be merged

## Acknowledgments

We appreciate the security research community's efforts in helping keep this project secure. Contributors who report valid security issues will be acknowledged here (with permission).
