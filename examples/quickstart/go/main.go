package main

import (
	osano "github.com/jflavan/pulumi-osano/sdk/go/osano"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		subjectRef := cfg.RequireSecret("subjectRef")
		configID := cfg.Require("configId")
		protocolID := cfg.Require("privacyProtocolId")
		subjectType := cfg.Get("subjectType")

		var jurisdiction pulumi.StringPtrInput
		if value := cfg.Get("jurisdiction"); value != "" {
			jurisdiction = pulumi.String(value)
		}

		subject := subjectRef.ApplyT(func(ref string) osano.ConsentSubject {
			if subjectType == "anonymous" {
				return osano.ConsentSubject{AnonymousId: &ref}
			}
			return osano.ConsentSubject{VerifiedId: &ref}
		}).(osano.ConsentSubjectOutput)

		consent, err := osano.NewConsent(ctx, "example-consent", &osano.ConsentArgs{
			Subject: subject,
			Actions: osano.ConsentActionArray{
				&osano.ConsentActionArgs{
					Target:       pulumi.String(protocolID),
					Vendor:       pulumi.String(configID),
					Action:       pulumi.String("ACCEPT"),
					Jurisdiction: jurisdiction,
				},
			},
			Attributes: pulumi.StringMap{
				"pulumiStack": pulumi.String(ctx.Stack()),
			},
			Origin: pulumi.String("api"),
			Tags: pulumi.StringArray{
				pulumi.String("demo"),
			},
		})
		if err != nil {
			return err
		}

		ctx.Export("consentId", consent.ConsentId)
		ctx.Export("lastSynced", consent.LastSynced)
		return nil
	})
}
