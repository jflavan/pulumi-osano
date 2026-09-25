package provider

import (
	"encoding/json"
	"strings"
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
	Name              string                                        `json:"name"`
	DisplayName       string                                        `json:"displayName"`
	License           string                                        `json:"license"`
	Publisher         string                                        `json:"publisher"`
	LogoURL           string                                        `json:"logoUrl"`
	Keywords          []string                                      `json:"keywords"`
	PluginDownloadURL string                                        `json:"pluginDownloadURL"`
	Config            struct{ Variables map[string]schemaProperty } `json:"config"`
	Resources         map[string]schemaObject                       `json:"resources"`
	Functions         map[string]schemaObject                       `json:"functions"`
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

// TestProviderSchemaRegistryMetadata checks the fields the Pulumi Registry's resourcedocsgen validates
// when it lists a community package: a publisher, exactly one known category/ keyword, and the
// kind/native keyword. The logo must be an absolute https URL the registry can load.
func TestProviderSchemaRegistryMetadata(t *testing.T) {
	t.Parallel()
	schema := loadProviderSchema(t)

	if schema.Name != "osano" || schema.DisplayName == "" {
		t.Fatalf("unexpected package name %q / displayName %q", schema.Name, schema.DisplayName)
	}
	if schema.Publisher == "" {
		t.Fatal("the registry requires a publisher in the schema")
	}
	if !strings.HasPrefix(schema.LogoURL, "https://") || !strings.HasSuffix(schema.LogoURL, ".png") {
		t.Fatalf("expected an https PNG logoUrl, got %q", schema.LogoURL)
	}
	if schema.PluginDownloadURL != "github://api.github.com/jflavan/pulumi-osano" {
		t.Fatalf("unexpected pluginDownloadURL %q", schema.PluginDownloadURL)
	}

	// The registry's category names (tools/resourcedocsgen/pkg/lookup.go CategoryNameMap).
	knownCategories := map[string]bool{
		"cloud": true, "database": true, "infrastructure": true, "monitoring": true,
		"network": true, "utility": true, "vcs": true,
	}
	var categories []string
	for _, keyword := range schema.Keywords {
		if category, ok := strings.CutPrefix(keyword, "category/"); ok {
			if !knownCategories[category] {
				t.Errorf("unknown registry category keyword %q", keyword)
			}
			categories = append(categories, category)
		}
	}
	if len(categories) != 1 {
		t.Errorf("expected exactly one category/ keyword, got %v", categories)
	}
	assertContains(t, schema.Keywords, "kind/native", "osano")
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
