module osano-quickstart-go

go 1.24

require (
	github.com/pulumi/pulumi/sdk/v3 v3.0.0
	github.com/jflavan/pulumi-osano/sdk/go/osano v0.0.0
)

replace github.com/jflavan/pulumi-osano/sdk/go/osano => ../../../sdk/go/osano
