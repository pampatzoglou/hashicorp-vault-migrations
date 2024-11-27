package migrations

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// VaultConfig holds Vault-specific configuration
type VaultConfig struct {
	Address     string `yaml:"address"`
	Token       string `yaml:"token"`
	AuthMethod  string `yaml:"auth_method,omitempty"`
	Role        string `yaml:"role,omitempty"`
	Namespace   string `yaml:"namespace,omitempty"`
	MaxRetries  int    `yaml:"max_retries,omitempty"`
	RetryDelay  string `yaml:"retry_delay,omitempty"`
}

// MigrationsConfig holds migration-specific configuration
type MigrationsConfig struct {
	Directory        string `yaml:"directory"`
	ConcurrentTasks bool   `yaml:"concurrent_tasks,omitempty"`
	StopOnError     bool   `yaml:"stop_on_error,omitempty"`
}

// Config holds the complete configuration
type Config struct {
	Vault      VaultConfig      `yaml:"vault"`
	Migrations MigrationsConfig `yaml:"migrations"`
	LogLevel   string          `yaml:"log_level,omitempty"`
	DryRun     bool           `yaml:"dry_run,omitempty"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set default values
	config := &Config{
		Vault: VaultConfig{
			MaxRetries: 3,
			RetryDelay: "1s",
		},
		Migrations: MigrationsConfig{
			ConcurrentTasks: true,
			StopOnError:     true,
		},
		LogLevel: "info",
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Environment variable interpolation
	config.Vault.Address = interpolateEnv(config.Vault.Address)
	config.Vault.Token = interpolateEnv(config.Vault.Token)
	config.Vault.Role = interpolateEnv(config.Vault.Role)
	config.Vault.Namespace = interpolateEnv(config.Vault.Namespace)

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if c.Vault.Address == "" {
		return fmt.Errorf("vault address is required")
	}

	if c.Vault.AuthMethod == "" && c.Vault.Token == "" {
		return fmt.Errorf("either vault token or auth method is required")
	}

	if c.Migrations.Directory == "" {
		return fmt.Errorf("migrations directory is required")
	}

	// Ensure migrations directory exists
	if _, err := os.Stat(c.Migrations.Directory); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory does not exist: %s", c.Migrations.Directory)
	}

	return nil
}

// interpolateEnv replaces environment variables in the format ${VAR} or $VAR
func interpolateEnv(value string) string {
	if value == "" {
		return value
	}

	// Replace ${VAR} format
	value = os.Expand(value, func(key string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		return "${" + key + "}"
	})

	// Replace $VAR format
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		value = strings.ReplaceAll(value, "$"+pair[0], pair[1])
	}

	return value
}
