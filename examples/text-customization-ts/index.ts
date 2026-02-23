import * as osano from "@pulumi/osano";

// Create a Cookie Consent configuration with customized en-US text.
const config = new osano.CookieConsentConfig("text-custom-config", {
    name: "text-customization-example",
    domains: ["example.com"],
    mode: "debug",
    configuration: {
        storagePolicyHref: "https://example.com/storage-policy",
        // Customize consent dialog text for en-US.
        text: {
            "en-US": {
                dialog: {
                    title: "We value your privacy",
                    description:
                        "We use cookies and similar technologies to provide you with the best experience. " +
                        "You can manage your preferences below.",
                    acceptAll: "Accept All",
                    denyAll: "Deny All",
                    save: "Save Preferences",
                },
                categories: {
                    ESSENTIAL: {
                        title: "Essential Cookies",
                        description: "Required for basic site functionality. Cannot be disabled.",
                    },
                    ANALYTICS: {
                        title: "Analytics Cookies",
                        description: "Help us understand how visitors interact with our website.",
                    },
                    MARKETING: {
                        title: "Marketing Cookies",
                        description: "Used to deliver relevant advertisements based on your interests.",
                    },
                    PERSONALIZATION: {
                        title: "Personalization Cookies",
                        description: "Allow the site to remember your preferences and choices.",
                    },
                },
            },
        },
    },
});

export const configId = config.configId;
