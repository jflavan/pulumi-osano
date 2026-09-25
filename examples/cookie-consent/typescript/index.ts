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

// Build the config from the same values hashed into changeToken, so any publish-relevant edit
// also changes the token and triggers exactly one republish.
const consentConfig = new osano.CookieConsentConfig("cookie-consent", {
    name: publishDescriptor.name,
    domains: publishDescriptor.domains,
    mode: publishDescriptor.mode,
    configuration: publishDescriptor.configuration,
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

// Downstream website resources consume the tag as an ordinary output. It must be the first script in
// <head>, with no async or defer attribute, so it can block tags that load after it. Pass headHtml
// (or the tag itself) to the resource that renders or configures the site, for example a template
// file, a CDN edge function, or a hosting provider's head-script setting.
export const headHtml = pulumi.interpolate`<head>
  ${publication.scriptTag}
  <meta charset="utf-8">
</head>`;
