---
title: Osano (Unofficial)
meta_desc: Use the unofficial Osano provider for Pulumi to manage Osano Cookie Consent configurations, rules, and publications and to submit and read Unified Consent records.
layout: package
---

The Osano (Unofficial) provider for Pulumi lets you manage [Osano](https://www.osano.com/) consent management as code. Create Cookie Consent (CMP) configurations and rules, publish them once they settle and get back the exact `<script>` tag to install, submit Unified Consent decisions, and read consent, subject, configuration, and privacy protocol data with functions.

This is a community-maintained provider. It is not affiliated with or endorsed by Osano, Inc. or Pulumi Corporation. Source code and issues are at [github.com/jflavan/pulumi-osano](https://github.com/jflavan/pulumi-osano).

## Installation

The provider is available as a package for TypeScript/JavaScript, Python, Go, .NET, and Java, and as a plugin for Pulumi YAML. Each SDK records where its provider plugin is published, so Pulumi downloads the matching plugin from the provider's GitHub releases the first time you run `pulumi preview` or `pulumi up`.

{{< chooser language "typescript,python,go,csharp,java,yaml,hcl" >}}
{{% choosable language typescript %}}

```bash
npm install @jflavan/pulumi-osano
```

{{% /choosable %}}
{{% choosable language python %}}

```bash
pip install pulumi-osano
```

{{% /choosable %}}
{{% choosable language go %}}

```bash
go get github.com/jflavan/pulumi-osano/sdk/go/osano
```

{{% /choosable %}}
{{% choosable language csharp %}}

```bash
dotnet add package Community.Pulumi.Osano
```

{{% /choosable %}}
{{% choosable language java %}}

Replace `VERSION` with the [latest release](https://github.com/jflavan/pulumi-osano/releases) (for example `0.1.0`).

Maven:

```xml
<dependency>
    <groupId>io.github.jflavan.pulumi</groupId>
    <artifactId>pulumi-osano</artifactId>
    <version>VERSION</version>
</dependency>
```

Gradle:

```groovy
implementation("io.github.jflavan.pulumi:pulumi-osano:VERSION")
```

{{% /choosable %}}
{{% choosable language yaml %}}

Pulumi YAML programs use the provider plugin directly. Install the release you want to use (for example `0.1.0`; see [releases](https://github.com/jflavan/pulumi-osano/releases)), then reference the resource types (for example `osano:index:CookieConsentConfig`) in your program:

```bash
pulumi plugin install resource osano VERSION --server github://api.github.com/jflavan/pulumi-osano
```

{{% /choosable %}}
{{% choosable language hcl %}}

This provider has not been tested with Pulumi HCL programs. Use one of the SDK languages or Pulumi YAML.

{{% /choosable %}}
{{< /chooser >}}

The [packages and publishing guide](https://github.com/jflavan/pulumi-osano/blob/main/docs/PUBLISHING.md) links each package's registry page and explains how to verify release checksums, signatures, and build provenance.

## Example Usage

The program below creates a Cookie Consent configuration. Cookie Consent resources authenticate with an Osano Customer REST API key:

```bash
pulumi config set --secret osano:osanoApiKey <customer-rest-api-key>
```

{{< chooser language "typescript,python,go,csharp,java" >}}
{{% choosable language typescript %}}

```typescript
import * as osano from "@jflavan/pulumi-osano";

const consentConfig = new osano.CookieConsentConfig("cookie-consent", {
    name: "example-cookie-consent",
    domains: ["example.com"],
    mode: "permissive",
    configuration: {
        storagePolicyHref: "https://example.com/privacy/cookies",
    },
});

export const configId = consentConfig.configId;
```

{{% /choosable %}}
{{% choosable language python %}}

```python
import pulumi
import pulumi_osano as osano

consent_config = osano.CookieConsentConfig(
    "cookie-consent",
    name="example-cookie-consent",
    domains=["example.com"],
    mode="permissive",
    configuration={
        "storagePolicyHref": "https://example.com/privacy/cookies",
    },
)

pulumi.export("configId", consent_config.config_id)
```

{{% /choosable %}}
{{% choosable language go %}}

```go
package main

import (
	"github.com/jflavan/pulumi-osano/sdk/go/osano"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		consentConfig, err := osano.NewCookieConsentConfig(ctx, "cookie-consent", &osano.CookieConsentConfigArgs{
			Name:    pulumi.String("example-cookie-consent"),
			Domains: pulumi.StringArray{pulumi.String("example.com")},
			Mode:    pulumi.String("permissive"),
			Configuration: pulumi.Map{
				"storagePolicyHref": pulumi.String("https://example.com/privacy/cookies"),
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("configId", consentConfig.ConfigId)
		return nil
	})
}
```

{{% /choosable %}}
{{% choosable language csharp %}}

```csharp
using System.Collections.Generic;
using Community.Pulumi.Osano;
using Pulumi;

return await Deployment.RunAsync(() =>
{
    var consentConfig = new CookieConsentConfig("cookie-consent", new()
    {
        Name = "example-cookie-consent",
        Domains = { "example.com" },
        Mode = "permissive",
        Configuration =
        {
            ["storagePolicyHref"] = "https://example.com/privacy/cookies",
        },
    });

    return new Dictionary<string, object?>
    {
        ["configId"] = consentConfig.ConfigId,
    };
});
```

{{% /choosable %}}
{{% choosable language java %}}

```java
package myproject;

import com.pulumi.Pulumi;
import io.github.jflavan.pulumi.osano.CookieConsentConfig;
import io.github.jflavan.pulumi.osano.CookieConsentConfigArgs;
import java.util.Map;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            var consentConfig = new CookieConsentConfig("cookie-consent", CookieConsentConfigArgs.builder()
                .name("example-cookie-consent")
                .domains("example.com")
                .mode("permissive")
                .configuration(Map.of("storagePolicyHref", "https://example.com/privacy/cookies"))
                .build());

            ctx.export("configId", consentConfig.configId());
        });
    }
}
```

{{% /choosable %}}
{{< /chooser >}}

A configuration is not live until it is published. Add a `CookieConsentPublication` that depends on the configuration and its `CookieConsentRule` resources; it publishes the configuration, waits for Osano to finish, and exports `scriptSrc` and `scriptTag`. The [Cookie Consent example](https://github.com/jflavan/pulumi-osano/tree/main/examples/cookie-consent) shows the complete program in C# and TypeScript, and the [end-to-end workflow guide](https://github.com/jflavan/pulumi-osano/blob/main/docs/end-to-end-workflow.md) covers day-2 changes, imports, and teardown.

## Configuration

Every setting is optional at the provider level. Each resource and function needs the API key for the Osano API it calls: Cookie Consent resources and the `sendSubjectCode` and `verifySubjectCode` functions need `osanoApiKey`, and the `Consent` resource and the other functions need `unifiedConsentApiKey`. Where an environment variable is listed, it takes precedence over the Pulumi configuration value when it is set.

- `osanoApiKey` (Optional, Secret) — Osano Customer REST API key, sent as `x-osano-api-key`. Used by Cookie Consent resources and the `sendSubjectCode` and `verifySubjectCode` functions. May also be set with the `OSANO_API_KEY` environment variable.
- `unifiedConsentApiKey` (Optional, Secret) — Unified Consent API key, sent as `x-uc-api-key`. Used by the `Consent` resource and the Unified Consent read functions. May also be set with the `OSANO_UC_API_KEY` environment variable.
- `apiBaseUrl` (Optional) — Base URL of the Unified Consent API, including any path prefix. Defaults to `https://uc.api.osano.com`. May also be set with the `OSANO_API_BASE_URL` environment variable.
- `customerBaseUrl` (Optional) — Base URL of the Customer REST API. Defaults to `https://api.osano.com`.
- `requestTimeoutSeconds` (Optional) — HTTP request timeout, in seconds, for Customer REST API and Unified Consent calls. Defaults to `60`. May also be set with the `OSANO_API_TIMEOUT_SECONDS` environment variable, which is used only when it is a positive integer.
- `ucApiKey` (Optional, Secret, Deprecated) — Former name of `unifiedConsentApiKey`, read only when `unifiedConsentApiKey` and `OSANO_UC_API_KEY` are not set. Use `unifiedConsentApiKey` instead.
- `ucBaseUrl` (Optional, Deprecated) — Former name of `apiBaseUrl`, read only when `apiBaseUrl` and `OSANO_API_BASE_URL` are not set. Use `apiBaseUrl` instead.

See [Installation & Configuration](https://www.pulumi.com/registry/packages/osano/installation-configuration/) for configuration examples.

## Resources and functions

| Resource | Purpose |
| --- | --- |
| `CookieConsentConfig` | A Cookie Consent (CMP) configuration: name, domains, mode, and configuration object. |
| `CookieConsentRule` | A cookie, script, iframe, or localStorage classification rule in a configuration. |
| `CookieConsentPublication` | Publishes a configuration when its `changeToken` changes and exports `scriptSrc` and `scriptTag`. |
| `Consent` | Submits a Unified Consent decision for a subject. |

Functions: `getUnifiedConsent`, `getSubject`, `getConfig`, `getCollections`, `getCollection`, `checkConsent`, `getConsentProfile`, `sendSubjectCode`, and `verifySubjectCode`. Pulumi runs functions on every preview, update, and refresh, so call `sendSubjectCode` and `verifySubjectCode` from automation rather than a long-lived stack; otherwise each run sends a new code.

## Lifecycle notes

- `pulumi preview`, `pulumi refresh`, and `pulumi import` never publish a Cookie Consent configuration. A `CookieConsentPublication` publishes when it is created and when an update changes its `changeToken`, `keepUnclassifiedTattles`, `description`, or `webhookUrl`. Derive `changeToken` from every publish-relevant value so that each real change publishes exactly once.
- `keepUnclassifiedTattles` defaults to `true`, so publishing does not delete unclassified discoveries.
- Deleting a `CookieConsentRule` deletes the rule in Osano. Osano has no delete or unpublish endpoint for configurations, so deleting a `CookieConsentConfig` or `CookieConsentPublication` only removes it from Pulumi state, and the configuration and its published script stay live.
- Consent records are immutable in Osano. Destroying a `Consent` resource removes it from Pulumi state only.
- Import configurations and publications with the Osano config ID, and rules with the composite ID `<configId>/<ruleId>`. Imports only read; an imported publication republishes once when your program applies its own `changeToken`.
