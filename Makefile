PACK            := osano
PROVIDER        := pulumi-resource-$(PACK)
PROJECT         := github.com/johnflavan/pulumi-osano
VERSION_PATH    := provider.Version
WORKING_DIR     := $(shell pwd)
SCHEMA_FILE     := provider/cmd/pulumi-resource-osano/schema.json

PROVIDER_VERSION ?= 0.0.1-dev

.PHONY: provider schema clean test

provider:
	cd provider && go build -o $(WORKING_DIR)/bin/$(PROVIDER) \
		-ldflags "-X $(PROJECT)/$(VERSION_PATH)=$(PROVIDER_VERSION)" \
		$(PROJECT)/provider/cmd/$(PROVIDER)

$(SCHEMA_FILE): provider
	pulumi package get-schema $(WORKING_DIR)/bin/$(PROVIDER) > $(SCHEMA_FILE)

schema: $(SCHEMA_FILE)

sdk/%: $(SCHEMA_FILE)
	rm -rf sdk/$*
	pulumi package gen-sdk --language $* $(SCHEMA_FILE) --version "$(PROVIDER_VERSION)"

test:
	cd provider && go test -race -v -count=1 ./...

clean:
	rm -rf bin/ sdk/dotnet sdk/go sdk/java sdk/nodejs sdk/python $(SCHEMA_FILE)
