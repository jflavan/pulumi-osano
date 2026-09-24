PROJECT_NAME := Pulumi Provider Osano

PACK             := osano
PACKDIR          := sdk
PROJECT          := github.com/jflavan/pulumi-osano
NODE_MODULE_NAME := @jflavan/pulumi-osano
NUGET_PKG_NAME   := Community.Pulumi.Osano

PROVIDER        := pulumi-resource-${PACK}
PROVIDER_PATH   := provider
VERSION_PATH    := ${PROVIDER_PATH}/version.Version

PULUMI          := pulumi

SCHEMA_FILE     := provider/cmd/pulumi-resource-osano/schema.json
export GOPATH   := $(shell go env GOPATH)

WORKING_DIR     := $(shell pwd)
TESTPARALLELISM := 4

# Override during CI using `make [TARGET] PROVIDER_VERSION=""` or by setting a PROVIDER_VERSION environment variable
# Local & branch builds will just used this fixed default version unless specified
PROVIDER_VERSION ?= 1.0.0-alpha.0+dev
# Use this normalised version everywhere rather than the raw input to ensure consistency.
VERSION_GENERIC = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)")

# Need to pick up locally pinned pulumi-langage-* plugins.
export PULUMI_IGNORE_AMBIENT_PLUGINS = true

ensure::
	go mod tidy

$(SCHEMA_FILE): provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/${PROVIDER} | \
		jq 'del(.version) | (.language.go.importBasePath="github.com/jflavan/pulumi-osano/sdk/go/osano")' > $(SCHEMA_FILE)

# Codegen generates the schema file and *generates* all sdks. This is a local process and
# does not require the ability to build all SDKs.
#
# To build the SDKs, use `make build_sdks`
#
# Required by CI (weekly-pulumi-update)
codegen: $(SCHEMA_FILE) sdk/dotnet sdk/go sdk/nodejs sdk/python sdk/java

.PHONY: sdk/%
sdk/%: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language $* $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/nodejs: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language nodejs $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md ${PACKDIR}/nodejs/

sdk/java: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language java $(SCHEMA_FILE)
	find sdk/java -name '*.java' -type f -exec perl -pi -e 's/[ \t]+$$//' {} +
	# Generated settings.gradle references a non-existent 'lib' module; drop it for a single-module build.
	@if [ "$$(uname)" = "Darwin" ]; then \
		sed -i '' '/^include("lib")/d' sdk/java/settings.gradle; \
	else \
		sed -i '/^include("lib")/d' sdk/java/settings.gradle; \
	fi
	# Post-process build.gradle for Maven Central publishing
	# pulumi-java-gen doesn't support all POM metadata fields, so we use a Python script for reliable patching
	@echo "Post-processing Java SDK build.gradle for Maven Central..."
	@python3 scripts/patch-java-build-gradle.py sdk/java/build.gradle

sdk/python: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	# Pulumi SDK generator doesn't set version in setup.py, so we patch it manually
	sed -i.bak 's/VERSION = "0.0.0"/VERSION = "${VERSION_GENERIC}"/' ${PACKDIR}/python/setup.py && rm ${PACKDIR}/python/setup.py.bak
	@python3 scripts/normalize-python-sdk.py ${PACKDIR}/python
	cp README.md ${PACKDIR}/python/

sdk/dotnet: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language dotnet $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md ${PACKDIR}/dotnet/
	@python3 scripts/patch-dotnet-csproj.py sdk/dotnet/Community.Pulumi.Osano.csproj



sdk/go: ${SCHEMA_FILE}
	rm -rf $@
	$(PULUMI) package gen-sdk --language go ${SCHEMA_FILE} --version "${VERSION_GENERIC}"
	GO_PKG_DIR=${PACKDIR}/go/osano; \
	mkdir -p $$GO_PKG_DIR; \
	cp go.mod $$GO_PKG_DIR/go.mod; \
	cd $$GO_PKG_DIR && \
		go mod edit -module=github.com/jflavan/pulumi-osano/sdk/go/osano && \
		go mod tidy

.PHONY: provider
provider: bin/${PROVIDER} bin/pulumi-gen-${PACK} # Required by CI

# Provider source files to track for rebuilds
PROVIDER_SRC := $(shell find provider -name '*.go')

bin/${PROVIDER}: $(PROVIDER_SRC)
	cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER)

.PHONY: provider_debug
provider_debug:
	(cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -gcflags="all=-N -l" -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER))

test_provider:
	cd provider && go test -short -race -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} -coverprofile="coverage.txt" ./...

dotnet_sdk: sdk/dotnet
	cd ${PACKDIR}/dotnet/&& \
		cp ../../README.md . && \
		echo "${VERSION_GENERIC}" > version.txt && \
		dotnet build

go_sdk:	sdk/go

nodejs_sdk: sdk/nodejs
	cd ${PACKDIR}/nodejs/ && \
		yarn install && \
		yarn run tsc
	cp README.md LICENSE ${PACKDIR}/nodejs/package.json ${PACKDIR}/nodejs/yarn.lock ${PACKDIR}/nodejs/bin/

python_sdk: sdk/python
	cp README.md ${PACKDIR}/python/
	cd ${PACKDIR}/python/ && \
		rm -rf ./bin/ ../python.bin/ && cp -R . ../python.bin && mv ../python.bin ./bin && \
		python3 -m venv venv && \
		./venv/bin/python -m pip install build && \
		cd ./bin && \
		../venv/bin/python -m build .

java_sdk:: PACKAGE_VERSION := $(VERSION_GENERIC)
java_sdk:: sdk/java
	cd sdk/java/ && \
		gradle --console=plain build

.PHONY: build
build:: provider build_sdks

.PHONY: build_sdks
build_sdks: dotnet_sdk go_sdk nodejs_sdk python_sdk java_sdk

.PHONY: build_cookie_consent_examples build_examples
build_cookie_consent_examples: dotnet_sdk nodejs_sdk
	dotnet build examples/cookie-consent/csharp/CookieConsent.csproj
	cd examples/cookie-consent/typescript && yarn install --frozen-lockfile && yarn run tsc --noEmit

.PHONY: build_quickstart_examples
build_quickstart_examples:
	cd examples/quickstart/go && go build -o /dev/null .

build_examples: build_cookie_consent_examples build_quickstart_examples

# Required for the codegen action that runs in pulumi/pulumi
only_build:: build

lint:
	golangci-lint --path-prefix provider --config .golangci.yml run --fix


install:: install_nodejs_sdk install_dotnet_sdk
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin


GO_TEST := go test -race -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM}

# Compiles every e2e build-tag set without running it, so a broken e2e suite fails without live credentials.
.PHONY: test_e2e_compile
test_e2e_compile:
	go vet -tags 'e2e consentread' ./tests/...
	go vet -tags 'e2e consentwrite' ./tests/...
	go vet -tags 'e2e subjectverification' ./tests/...

.PHONY: test_scripts
test_scripts:
	python3 -m unittest discover -s scripts -p 'test_*.py'

test_all:: test test_e2e_compile test_scripts

install_dotnet_sdk::
	rm -rf $(WORKING_DIR)/nuget/$(NUGET_PKG_NAME).*.nupkg
	mkdir -p $(WORKING_DIR)/nuget
	find . -name '*.nupkg' -print -exec cp -p {} ${WORKING_DIR}/nuget \;

install_python_sdk::
	#target intentionally blank

install_go_sdk::
	#target intentionally blank

install_java_sdk::
	#target intentionally blank

install_nodejs_sdk::
	-yarn unlink --cwd $(WORKING_DIR)/sdk/nodejs/bin
	yarn link --cwd $(WORKING_DIR)/sdk/nodejs/bin

test:: test_provider

.PHONY:local_generate
local_generate: # Required by CI

.PHONY: generate_schema
generate_schema: ${SCHEMA_FILE} # Required by CI

.PHONY: build_go install_go_sdk
generate_go: sdk/go # Required by CI
build_go: # Required by CI

.PHONY: build_java install_java_sdk
generate_java: sdk/java # Required by CI
build_java: java_sdk # Required by CI

.PHONY: build_python install_python_sdk
generate_python: sdk/python # Required by CI
build_python: python_sdk # Required by CI

.PHONY: build_nodejs install_nodejs_sdk
generate_nodejs: sdk/nodejs # Required by CI
build_nodejs: nodejs_sdk # Required by CI

.PHONY: build_dotnet install_dotnet_sdk
generate_dotnet: sdk/dotnet # Required by CI
build_dotnet: dotnet_sdk # Required by CI

bin/pulumi-gen-${PACK}: # Required by CI
	touch bin/pulumi-gen-${PACK}
