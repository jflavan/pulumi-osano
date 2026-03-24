import * as pulumi from "@pulumi/pulumi";
import * as osano from "@jflavan/pulumi-osano";

const config = new pulumi.Config();
const subjectRef = config.requireSecret("subjectRef");
const configId = config.require("configId");
const privacyProtocolId = config.require("privacyProtocolId");
const subjectType = config.get("subjectType") ?? "verified";
const jurisdiction = config.get("jurisdiction") ?? undefined;

const subject = subjectRef.apply((value) => {
    if (subjectType === "anonymous") {
        return { anonymousId: value };
    }
    return { verifiedId: value };
});

const consent = new osano.Consent("example-consent", {
    subject,
    actions: [
        {
            target: privacyProtocolId,
            vendor: configId,
            action: "ACCEPT",
            jurisdiction,
        },
    ],
    attributes: {
        pulumiStack: pulumi.getStack(),
    },
    origin: "api",
    tags: ["demo"],
});

export const consentId = consent.consentId;
export const lastSynced = consent.lastSynced;
