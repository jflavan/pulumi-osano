import * as osano from "@pulumi/osano";

// Create a Cookie Consent configuration.
const config = new osano.CookieConsentConfig("example-config", {
    name: "example-consent-config",
    domains: ["example.com"],
    mode: "debug",
    configuration: {
        storagePolicyHref: "https://example.com/storage-policy",
    },
});

// Create a rule within the configuration.
const rule = new osano.CookieConsentRule("example-rule", {
    configId: config.configId,
    storeType: "scripts",
    classification: "ANALYTICS",
    rule: "google-analytics*",
    disclosure: true,
    title: "Google Analytics",
    vendorName: "Google",
});

export const configId = config.configId;
export const ruleId = rule.ruleId;
