package migrations

import (
	"github.com/hashicorp/vault/api"
)

// NewVaultClient initializes a new Vault client based on the configuration.
func NewVaultClient(config *Config) (*api.Client, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = config.Vault.Address

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, err
	}

	client.SetToken(config.Vault.Token)
	return client, nil
}
