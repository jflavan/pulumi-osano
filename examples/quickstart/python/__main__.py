import pulumi
import pulumi_osano as osano

config = pulumi.Config()
subject_ref = config.require_secret("subjectRef")
config_id = config.require("configId")
privacy_protocol_id = config.require("privacyProtocolId")
subject_type = config.get("subjectType") or "verified"
jurisdiction = config.get("jurisdiction")


def build_subject(value: str) -> osano.ConsentSubjectArgs:
    if subject_type == "anonymous":
        return osano.ConsentSubjectArgs(anonymous_id=value)
    return osano.ConsentSubjectArgs(verified_id=value)


subject = subject_ref.apply(build_subject)

consent = osano.Consent(
    "example-consent",
    subject=subject,
    actions=[
        osano.ConsentActionArgs(
            target=privacy_protocol_id,
            vendor=config_id,
            action="ACCEPT",
            jurisdiction=jurisdiction,
        )
    ],
    attributes={"pulumiStack": pulumi.get_stack()},
    origin="api",
    tags=["demo"],
)

pulumi.export("consentId", consent.consent_id)
pulumi.export("lastSynced", consent.last_synced)
