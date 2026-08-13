# Cookie Consent publication

These examples create an Osano Cookie Consent configuration and its managed
rules, publish that desired state, wait for Osano to report completion, and
export the hosted CMP script URL and complete script tag. C# is the canonical
example; TypeScript is a companion for this repository's baseline language
policy.

Compilation, local provider installation, and `pulumi preview` do not publish
to Osano. Only an opted-in `pulumi up` creates the configuration and rules and
then queues publication. The publication depends on the configuration and
every rule. Its deterministic `changeToken` is the SHA-256 hash of the complete
desired configuration/rule descriptor, so an unchanged `pulumi up` does not
publish again.

## C# from a clone

The committed C# project references the generated SDK at
`sdk/dotnet/Community.Pulumi.Osano.csproj`. From the repository root:

```bash
mise exec -- make build_cookie_consent_examples PROVIDER_VERSION=1.0.0-alpha.0+dev
cd examples/cookie-consent/csharp
pulumi stack init dev
```

The Make target builds the generated .NET and Node.js SDKs, creates ignored
local artifacts including `sdk/dotnet/version.txt` and `sdk/nodejs/bin`, and
compiles both examples. It does not install a provider plugin, run a Pulumi
deployment, or contact Osano.

## C# with the released NuGet package

To use the example outside this clone, copy the `csharp` directory and replace
its local `<ProjectReference>` with the released package reference:

```xml
<PackageReference Include="Community.Pulumi.Osano" Version="RELEASED_VERSION" />
```

Keep the existing Pulumi package reference, then run `dotnet restore` and
`dotnet build`, followed by `pulumi stack init dev`. Use an actual published
version in place of `RELEASED_VERSION`. In this released-package workflow, the
SDK requests its matching released provider plugin and Pulumi downloads that
plugin automatically when the program runs; do not install the checkout's dev
binary for it.

## TypeScript companion

The TypeScript project references the generated Node.js SDK at
`sdk/nodejs/bin` and uses the same desired values, deterministic SHA-256 token,
dependencies, and 20-minute create/update timeouts:

```bash
mise exec -- make build_cookie_consent_examples PROVIDER_VERSION=1.0.0-alpha.0+dev
cd examples/cookie-consent/typescript
pulumi stack init dev
```

Run the Make command from the repository root. It materializes the ignored
`sdk/nodejs/bin` package before the example's frozen install and TypeScript
compile, as described in the C# section, without publishing to Osano.

## Opt-in live smoke test

The following commands create and publish real resources in the Osano customer
account associated with the API key. Do not run them against an account where
that is not intended.

For either checkout/dev example, return to the repository root after the clone
setup above and build and install the provider at the same exact version as the
local SDK:

```bash
mise exec -- make provider PROVIDER_VERSION=1.0.0-alpha.0+dev
mise exec -- pulumi plugin install resource osano 1.0.0-alpha.0+dev --file ./bin/pulumi-resource-osano --exact --reinstall
cd examples/cookie-consent/csharp # or examples/cookie-consent/typescript
```

The explicit `--file` installs the just-built executable rather than
downloading a provider. These commands do not contact Osano. They deliberately
avoid `make install`, whose SDK linking/package-copy steps are unrelated to
this example. If you are using the released NuGet workflow instead, skip these
checkout-only commands; Pulumi downloads the matching released plugin declared
by that SDK.

From the selected example directory, set the inputs and explicitly deploy:

```bash
export OSANO_API_KEY="replace-with-a-customer-rest-api-key"
pulumi config set domain example.com
pulumi config set storagePolicyHref https://example.com/privacy/cookies
pulumi config set mode permissive
pulumi up
pulumi stack output cookieConsentScriptTag
```

`pulumi up` first creates or updates the configuration and managed rules, then
queues publication and waits for the accepted operation. Osano/CDN propagation
may continue for up to 15 minutes after the API reports the configuration as
published.

The output has this form, with no `async` or `defer` attribute:

```html
<script src="https://cmp.osano.com/CUSTOMER_ID/CONFIG_ID/osano.js"></script>
```

Place that tag first in the site's `<head>` so consent controls load before
other scripts.

## Destroy behavior

`pulumi destroy` deletes the managed rules from Osano. Osano exposes no
delete/unpublish endpoint for the configuration or publication, so removing
those Pulumi resources is state-only: the upstream configuration and its
publication remain active. Remove or disable retained customer resources in
Osano separately when required.
