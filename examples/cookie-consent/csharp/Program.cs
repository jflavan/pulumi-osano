using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Community.Pulumi.Osano;
using Pulumi;

return await Deployment.RunAsync(() =>
{
    var settings = new Pulumi.Config();
    var domain = settings.Require("domain");
    var storagePolicyHref = settings.Require("storagePolicyHref");
    var mode = settings.Get("mode") ?? "permissive";

    var cmpSettings = new SortedDictionary<string, object>
    {
        ["managePreferencesEnabled"] = true,
        ["storagePolicyHref"] = storagePolicyHref,
    };
    var ruleDefinitions = new[]
    {
        new
        {
            Name = "google-analytics-cookie",
            StoreType = "cookies",
            Classification = "ANALYTICS",
            Rule = "_ga",
            Disclosure = true,
            Title = "Google Analytics",
            VendorName = "Google",
            RuleType = "EXACT_MATCH",
            Description = "Measures site usage.",
            Expiry = "2 years",
        },
    };
    var publishDescriptor = new
    {
        Name = "pulumi-cookie-consent",
        Domains = new[] { domain },
        Mode = mode,
        Configuration = cmpSettings,
        Rules = ruleDefinitions,
    };
    var changeToken = Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(
        JsonSerializer.Serialize(publishDescriptor)
    ))).ToLowerInvariant();

    var consentConfig = new CookieConsentConfig("cookie-consent", new()
    {
        Name = publishDescriptor.Name,
        Domains = { domain },
        Mode = mode,
        Configuration =
        {
            { "managePreferencesEnabled", true },
            { "storagePolicyHref", storagePolicyHref },
        },
    });

    var rules = ruleDefinitions.Select(definition => new CookieConsentRule(definition.Name, new()
    {
        ConfigId = consentConfig.ConfigId,
        StoreType = definition.StoreType,
        Classification = definition.Classification,
        Rule = definition.Rule,
        Disclosure = definition.Disclosure,
        Title = definition.Title,
        VendorName = definition.VendorName,
        RuleType = definition.RuleType,
        Description = definition.Description,
        Expiry = definition.Expiry,
    })).ToArray();

    var publicationOptions = new CustomResourceOptions
    {
        CustomTimeouts = new CustomTimeouts
        {
            Create = TimeSpan.FromMinutes(20),
            Update = TimeSpan.FromMinutes(20),
        },
    };
    publicationOptions.DependsOn.Add(consentConfig);
    foreach (var rule in rules)
    {
        publicationOptions.DependsOn.Add(rule);
    }
    var publication = new CookieConsentPublication("publication", new()
    {
        ConfigId = consentConfig.ConfigId,
        ChangeToken = changeToken,
        KeepUnclassifiedTattles = true,
    }, publicationOptions);

    return new Dictionary<string, object?>
    {
        ["cookieConsentScriptSrc"] = publication.ScriptSrc,
        ["cookieConsentScriptTag"] = publication.ScriptTag,
    };
});
