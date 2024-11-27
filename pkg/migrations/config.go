package migrations

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config defines the configuration structure.
type Config struct {
	Vault       VaultConfig       `yaml:"vault"`
	Migrations  MigrationsConfig  `yaml:"migrations"`
}

// VaultConfig defines Vault-related configuration.
type VaultConfig struct {
	Address string `yaml:"address"`
	Token   string `yaml:"tokenFile"` // Token file instead of direct env var.
}

// MigrationsConfig defines migrations-related configuration.
type MigrationsConfig struct {
	Directory string `yaml:"directory"`
}

// LoadConfig loads the configuration from a YAML file.
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Load token from file
	token, err := os.ReadFile(config.Vault.Token)
	if err != nil {
		return nil, err
	}
	config.Vault.Token = string(token)

	return &config, nil
}
