package provider

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
)

type schemaProperty struct {
	Description string `json:"description"`
	Secret      bool   `json:"secret"`
	Default     any    `json:"default"`
}

type schemaObject struct {
	Description     string                    `json:"description"`
	InputProperties map[string]schemaProperty `json:"inputProperties"`
	Properties      map[string]schemaProperty `json:"properties"`
	RequiredInputs  []string                  `json:"requiredInputs"`
	Required        []string                  `json:"required"`
	Inputs          struct {
		Properties map[string]schemaProperty `json:"properties"`
	} `json:"inputs"`
}

type providerSchema struct {
	License   string                                        `json:"license"`
	Config    struct{ Variables map[string]schemaProperty } `json:"config"`
	Resources map[string]schemaObject                       `json:"resources"`
	Functions map[string]schemaObject                       `json:"functions"`
}

func loadProviderSchema(t *testing.T) providerSchema {
	t.Helper()
	server, err := integration.NewServer(
		t.Context(), Name, semver.MustParse("1.0.0"), integration.WithProvider(Provider()),
	)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var schema providerSchema
	if err := json.Unmarshal([]byte(resp.Schema), &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestProviderSchemaContract(t *testing.T) {
	t.Parallel()
	schema := loadProviderSchema(t)

	if schema.License != "MIT" {
		t.Fatalf("expected MIT license, got %q", schema.License)
	}

	for _, key := range []string{"osanoApiKey", "unifiedConsentApiKey", "ucApiKey"} {
		if !schema.Config.Variables[key].Secret {
			t.Fatalf("expected config %s to be secret", key)
		}
	}

	publication, ok := schema.Resources["osano:index:CookieConsentPublication"]
	if !ok {
		t.Fatal("CookieConsentPublication missing from schema")
	}
	assertContains(t, publication.RequiredInputs, "configId", "changeToken")
	assertContains(t, publication.Required, "scriptSrc", "scriptTag")
	if got := publication.InputProperties["keepUnclassifiedTattles"].Default; got != true {
		t.Fatalf("expected keepUnclassifiedTattles default true, got %#v", got)
	}
	for _, output := range []string{"scriptSrc", "scriptTag"} {
		if publication.Properties[output].Secret {
			t.Fatalf("public script output %s must not be secret", output)
		}
	}

	rule := schema.Resources["osano:index:CookieConsentRule"]
	for _, input := range []string{"ruleType", "description", "expiry"} {
		if _, ok := rule.InputProperties[input]; !ok {
			t.Fatalf("CookieConsentRule missing %s input", input)
		}
	}

	if !schema.Functions["osano:index:verifySubjectCode"].Inputs.Properties["code"].Secret {
		t.Fatal("expected verifySubjectCode.code to be secret")
	}
}

func TestProviderSchemaDescribesEveryInput(t *testing.T) {
	t.Parallel()
	schema := loadProviderSchema(t)
	if len(schema.Resources) != 4 || len(schema.Functions) != 9 || len(schema.Config.Variables) == 0 {
		t.Fatalf("unexpected schema shape: %d resources, %d functions, %d config variables",
			len(schema.Resources), len(schema.Functions), len(schema.Config.Variables))
	}

	for token, resource := range schema.Resources {
		if resource.Description == "" {
			t.Errorf("resource %s has no description", token)
		}
		for name, prop := range resource.InputProperties {
			if prop.Description == "" {
				t.Errorf("resource %s input %s has no description", token, name)
			}
		}
	}
	for token, function := range schema.Functions {
		if function.Description == "" {
			t.Errorf("function %s has no description", token)
		}
		for name, prop := range function.Inputs.Properties {
			if prop.Description == "" {
				t.Errorf("function %s input %s has no description", token, name)
			}
		}
	}
	for name, variable := range schema.Config.Variables {
		if variable.Description == "" {
			t.Errorf("config %s has no description", name)
		}
	}
}

func assertContains(t *testing.T, values []string, want ...string) {
	t.Helper()
	present := map[string]bool{}
	for _, value := range values {
		present[value] = true
	}
	for _, value := range want {
		if !present[value] {
			t.Fatalf("expected %q in %v", value, values)
		}
	}
}
