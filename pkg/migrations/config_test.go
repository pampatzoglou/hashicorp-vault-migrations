package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "vault-migrations-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test config file
	configContent := `
vault:
  address: "http://vault:8200"
  token: "${VAULT_TOKEN}"
  auth_method: "token"
  namespace: "test-namespace"
  max_retries: 3
  retry_delay: "1s"

migrations:
  directory: "./migrations"
  concurrent_tasks: true
  stop_on_error: true

log_level: "info"
dry_run: false
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set test environment variables
	os.Setenv("VAULT_TOKEN", "test-token")
	defer os.Unsetenv("VAULT_TOKEN")

	// Test loading config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify config values
	assert.Equal(t, "http://vault:8200", config.Vault.Address)
	assert.Equal(t, "test-token", config.Vault.Token)
	assert.Equal(t, "token", config.Vault.AuthMethod)
	assert.Equal(t, "test-namespace", config.Vault.Namespace)
	assert.Equal(t, 3, config.Vault.MaxRetries)
	assert.Equal(t, "1s", config.Vault.RetryDelay)
	assert.Equal(t, "./migrations", config.Migrations.Directory)
	assert.True(t, config.Migrations.ConcurrentTasks)
	assert.True(t, config.Migrations.StopOnError)
	assert.Equal(t, "info", config.LogLevel)
	assert.False(t, config.DryRun)
}

func TestLoadConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "valid config",
			config: `
vault:
  address: "http://vault:8200"
  token: "test-token"
migrations:
  directory: "./migrations"
`,
			expectError: false,
		},
		{
			name: "missing vault address",
			config: `
vault:
  token: "test-token"
migrations:
  directory: "./migrations"
`,
			expectError: true,
		},
		{
			name: "missing auth credentials",
			config: `
vault:
  address: "http://vault:8200"
migrations:
  directory: "./migrations"
`,
			expectError: true,
		},
		{
			name: "missing migrations directory",
			config: `
vault:
  address: "http://vault:8200"
  token: "test-token"
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir, err := os.MkdirTemp("", "vault-migrations-config-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configPath, []byte(tt.config), 0644)
			require.NoError(t, err)

			// Test loading config
			_, err = LoadConfig(configPath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_EnvironmentInterpolation(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_VAULT_ADDR", "http://test-vault:8200")
	os.Setenv("TEST_VAULT_TOKEN", "test-token-123")
	os.Setenv("TEST_VAULT_NAMESPACE", "test-ns")
	defer func() {
		os.Unsetenv("TEST_VAULT_ADDR")
		os.Unsetenv("TEST_VAULT_TOKEN")
		os.Unsetenv("TEST_VAULT_NAMESPACE")
	}()

	// Create test config with environment variables
	configContent := `
vault:
  address: "${TEST_VAULT_ADDR}"
  token: "${TEST_VAULT_TOKEN}"
  namespace: "${TEST_VAULT_NAMESPACE}"
migrations:
  directory: "./migrations"
`
	tmpDir, err := os.MkdirTemp("", "vault-migrations-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test loading config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify environment variable interpolation
	assert.Equal(t, "http://test-vault:8200", config.Vault.Address)
	assert.Equal(t, "test-token-123", config.Vault.Token)
	assert.Equal(t, "test-ns", config.Vault.Namespace)
}

func TestConfig_DefaultValues(t *testing.T) {
	// Create minimal config
	configContent := `
vault:
  address: "http://vault:8200"
  token: "test-token"
migrations:
  directory: "./migrations"
`
	tmpDir, err := os.MkdirTemp("", "vault-migrations-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test loading config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify default values
	assert.Equal(t, 3, config.Vault.MaxRetries)
	assert.Equal(t, "1s", config.Vault.RetryDelay)
	assert.True(t, config.Migrations.ConcurrentTasks)
	assert.True(t, config.Migrations.StopOnError)
	assert.Equal(t, "info", config.LogLevel)
	assert.False(t, config.DryRun)
}
