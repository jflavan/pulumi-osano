# Version Control Practices

Pulumi lets you treat consent automation as code. Follow these practices to keep history clean and auditable.

1. **One stack per environment** – for example `osano-dev`, `osano-staging`, `osano-prod`. Each stack references its own API keys.
2. **Secrets in config** – never commit keys. Use `pulumi config set --secret` locally and `pulumi config refresh` inside CI to pull encrypted values from the Pulumi Service.
3. **Short-lived feature branches** – experiment in branches, run `pulumi preview`, then open a PR. Merge to main only after CI (lint + tests) pass.
4. **Code reviews** – treat provider changes like any other infrastructure change. Reviewers should confirm that new resources deliberately target the correct Osano configuration IDs.
5. **Tag releases** – tag the repo for every provider publish (`vX.Y.Z`) so downstream automation can lock to exact versions.
6. **Record metadata** – include Pulumi stack outputs that reference deployment IDs or commit SHAs so you can correlate deployments with Osano consent logs.

These habits make audits easier and reduce the risk of pushing test consent data into production systems.
