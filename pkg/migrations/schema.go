package migrations

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
)

// Schema represents the desired state of Vault configuration
type Schema struct {
	DesiredState map[string]interface{} `yaml:"desired_state"`
}

// LoadSchema loads and parses a schema file
func LoadSchema(schemaPath string) (*Schema, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema file: %w", err)
	}

	if schema.DesiredState == nil {
		return nil, fmt.Errorf("schema file must contain desired_state")
	}

	return &schema, nil
}

// sanitizeFilename ensures filenames are safe and standardized.
func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[^\w\-.]+`)
	return strings.ToLower(re.ReplaceAllString(name, "_"))
}

// ExampleVaultAPICall demonstrates a basic API interaction.
func ExampleVaultAPICall(client *api.Client, path string) (map[string]interface{}, error) {
	secret, err := client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read from Vault path %s: %w", path, err)
	}
	if secret == nil {
		return nil, fmt.Errorf("no data found at path %s", path)
	}
	return secret.Data, nil
}
