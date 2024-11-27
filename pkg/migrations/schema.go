package migrations

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/vault/api"
)

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
