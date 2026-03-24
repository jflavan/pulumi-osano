# Example Guidelines for pulumi-osano

This document explains how examples in the Pulumi Osano provider should be organized, documented, and maintained.

## Why Examples Matter

- They double as documentation for users learning the provider.
- They validate APIs by exercising real Pulumi programs.
- They provide regression coverage for future changes (manual or automated).

## Baseline Requirements

Every resource or invoke exported by the provider MUST have:

- ✅ A runnable **TypeScript** example.
- ✅ An accompanying `README.md` describing prerequisites, config, and outputs.
- ✅ Clearly named placeholder values (e.g., `your-privacy-protocol-id`).

When adding new features, update or create examples in the same PR.

## Recommended Coverage Levels

| Tier | Applies To | Requirement |
| --- | --- | --- |
| Tier 1 (Required) | All resources + invokes | TypeScript example + README |
| Tier 2 (Preferred) | Frequently used APIs | Add Python and Go variants |
| Tier 3 (Nice to have) | Mission-critical / complex workflows | Include .NET and Java samples or integration-style demos |

## Repository Layout

```
examples/
  quickstart/
    README.md
    typescript/
    python/
    go/
  <future-resource>/
    README.md
    typescript/
    ...
```

Each language folder includes its own `Pulumi.yaml` plus ecosystem-specific metadata (`package.json`, `requirements.txt`, `go.mod`, etc.).

## README Template

```markdown
# <Name> Example

This example demonstrates how to use the `osano:<package>:<resource>` resource (or invoke).

## What It Does

<Short description>

## Prerequisites

- Pulumi CLI + language runtime
- `OSANO_UC_API_KEY` (and other config as needed)
- Pulumi stack config commands to set secrets

## Running

```bash
cd typescript
npm install
pulumi up
```

Repeat for each language offered.

## Key Properties

- `privacyProtocolId` – …
- `subjectAnonymousId` – …
```

## Code Quality Checklist

- [ ] Minimal, declarative Pulumi programs (no unnecessary helpers).
- [ ] Helpful inline comments where behavior might surprise newcomers.
- [ ] Outputs that demonstrate meaningful data returned by the provider.
- [ ] Configurable inputs retrieved via `pulumi.Config` (avoid hardcoding secrets).
- [ ] `README.md` validated against actual steps required to run the program.

## Current Coverage Snapshot

| Area | Languages | Notes |
| --- | --- | --- |
| `examples/quickstart` | TypeScript, Python, Go | Demonstrates the `osano:index:Consent` resource and unified consent invoke |

Add new rows as additional resources or workflows are introduced.

## Contribution Workflow

1. Build / update provider code.
2. Create or update the example directory.
3. Run the example locally (`pulumi preview` at minimum) to ensure it succeeds.
4. Document any new config keys in the example README.
5. Commit the example code alongside provider changes and regenerated SDKs.

## Troubleshooting

- Use `pulumi config` for stack-scoped data (API keys, subject IDs, etc.).
- When referencing local SDK builds during development, set `PULUMI_PYTHONPATH`, `NODE_PATH`, or `GOMODCACHE` as needed or rely on `pulumi plugin install --local`.
- If an example requires multiple resources, prefer separate files over large monoliths so users can quickly see the relevant snippet.

## Questions?

Open a discussion at https://github.com/jflavan/pulumi-osano/discussions if you need help designing or validating an example.
