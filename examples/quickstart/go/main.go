package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	osano "github.com/jflavan/pulumi-osano/sdk/go/osano"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := ctx.Config()
		subjectRef := config.RequireSecret("subjectRef")
		configID := config.Require("configId")
		protocolID := config.Require("privacyProtocolId")
		subjectType := config.Get("subjectType")
		jurisdiction := config.Get("jurisdiction")

		subject := pulumi.All(subjectRef).ApplyT(func(values []interface{}) osano.ConsentSubject {
			ref := values[0].(string)
			if subjectType == "anonymous" {
				return osano.ConsentSubject{AnonymousId: pulumi.StringPtr(ref)}
			}
			return osano.ConsentSubject{VerifiedId: pulumi.StringPtr(ref)}
		}).(osano.ConsentSubjectOutput)

		consent, err := osano.NewConsent(ctx, "example-consent", &osano.ConsentArgs{
			Subject: subject,
			Actions: osano.ConsentActionArray{
				&osano.ConsentActionArgs{
					Target:       pulumi.String(protocolID),
					Vendor:       pulumi.String(configID),
					Action:       pulumi.String("ACCEPT"),
					Jurisdiction: pulumi.StringPtr(jurisdiction),
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
