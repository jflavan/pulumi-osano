# Version Control Practices

Pulumi lets you treat consent automation as code. Follow these practices to keep history clean and auditable.

1. **One stack per environment** – for example `osano-dev`, `osano-staging`, `osano-prod`. Each stack references its own API keys.
2. **Secrets in config** – never commit plaintext keys. `pulumi config set --secret` stores them encrypted in `Pulumi.<stack>.yaml`, which CI decrypts with the stack's secrets provider; alternatively inject `OSANO_API_KEY` / `OSANO_UC_API_KEY` from your CI secret store or Pulumi ESC.
3. **Short-lived feature branches** – experiment in branches, run `pulumi preview`, then open a PR. Merge to main only after CI (lint + tests) pass.
4. **Code reviews** – treat provider changes like any other infrastructure change. Reviewers should confirm that new resources deliberately target the correct Osano configuration IDs.
5. **Pin the provider version** – pin the Osano SDK package to an exact version and commit your lockfile, so every stack and CI run uses the same provider plugin. Upgrade deliberately; see [Pinning pre-1.0 releases](UPGRADE.md#pinning-pre-10-releases).
6. **Record metadata** – include Pulumi stack outputs that reference deployment IDs or commit SHAs so you can correlate deployments with Osano consent logs.

These habits make audits easier and reduce the risk of pushing test consent data into production systems.
