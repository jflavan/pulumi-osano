# Frequently Asked Questions

**Is this an official Osano integration?**
> No. This is a community-maintained Pulumi provider. It uses Osano's public Unified Consent API but is not affiliated with Osano.

**Which APIs are supported today?**
> The current release supports Unified Consent submission plus helper invokes for unified consent lookups, subject resolution, config and collection reads, consent profile reads, and subject verification. Managed configuration resources are still future work.

**Can I import existing consent records?**
> Consents are immutable log entries inside Osano, so there is nothing to import. See `docs/IMPORTING.md` for guidance on adopting the provider in existing environments.

**Do I need both API keys?**
> Most consent submissions only need the Unified Consent API key (`osano:unifiedConsentApiKey`). Administrative routes (subjects, profiles, merges) require the Osano API key (`osano:osanoApiKey`). Provide both if you plan to mix workloads.

**How are API errors surfaced?**
> The provider unwraps HTTP status codes and includes the response body in the Pulumi diagnostic, making it easy to map back to Osano's documentation.

**Does this provider store personal data in state?**
> Pulumi will store whatever inputs you supply (subject IDs, tags, attributes). Use Pulumi secrets (`--secret`) for anything sensitive.

**Where can I ask more questions?**
> Open a GitHub Discussion or start a thread in the Issues tab. Please avoid sharing real subject identifiers or API keys.
