import { createHash } from "crypto";
import * as pulumi from "@pulumi/pulumi";
import * as osano from "@jflavan/pulumi-osano";

const settings = new pulumi.Config();
const domain = settings.require("domain");
const storagePolicyHref = settings.require("storagePolicyHref");
const mode = settings.get("mode") ?? "permissive";

const cmpSettings = {
    managePreferencesEnabled: true,
    storagePolicyHref,
};
const ruleDefinitions = [{
    name: "google-analytics-cookie",
    storeType: "cookies",
    classification: "ANALYTICS",
    rule: "_ga",
    disclosure: true,
    title: "Google Analytics",
    vendorName: "Google",
    ruleType: "EXACT_MATCH",
    description: "Measures site usage.",
    expiry: "2 years",
}];
const publishDescriptor = {
    name: "pulumi-cookie-consent",
    domains: [domain],
    mode,
    configuration: cmpSettings,
    rules: ruleDefinitions,
};
const changeToken = createHash("sha256")
    .update(JSON.stringify(publishDescriptor))
    .digest("hex");

const consentConfig = new osano.CookieConsentConfig("cookie-consent", {
    name: publishDescriptor.name,
    domains: [domain],
    mode,
    configuration: cmpSettings,
});

const rules = ruleDefinitions.map((definition) => new osano.CookieConsentRule(definition.name, {
    configId: consentConfig.configId,
    storeType: definition.storeType,
    classification: definition.classification,
    rule: definition.rule,
    disclosure: definition.disclosure,
    title: definition.title,
    vendorName: definition.vendorName,
    ruleType: definition.ruleType,
    description: definition.description,
    expiry: definition.expiry,
}));

const publication = new osano.CookieConsentPublication("publication", {
    configId: consentConfig.configId,
    changeToken,
    keepUnclassifiedTattles: true,
}, {
    dependsOn: [consentConfig, ...rules],
    customTimeouts: { create: "20m", update: "20m" },
});

export const cookieConsentScriptSrc = publication.scriptSrc;
export const cookieConsentScriptTag = publication.scriptTag;
