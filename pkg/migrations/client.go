package migrations

import (
	"fmt"

	"github.com/hashicorp/vault/api"
)

// VaultClient wraps the Vault API client with additional functionality
type VaultClient struct {
	client *api.Client
}

// NewVaultClient initializes a new Vault client based on the configuration.
func NewVaultClient(config VaultConfig) (*VaultClient, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = config.Address

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	client.SetToken(config.Token)
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	return &VaultClient{
		client: client,
	}, nil
}

// GetClient returns the underlying Vault API client
func (c *VaultClient) GetClient() *api.Client {
	return c.client
}

// GetCurrentState retrieves the current state of Vault configuration
func (c *VaultClient) GetCurrentState() (map[string]interface{}, error) {
	// Initialize the state map
	state := make(map[string]interface{})

	// Get auth methods
	auths, err := c.client.Sys().ListAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to list auth methods: %w", err)
	}
	for path, auth := range auths {
		state[fmt.Sprintf("sys/auth/%s", path)] = map[string]interface{}{
			"type":        auth.Type,
			"description": auth.Description,
		}
	}

	// Get policies
	policies, err := c.client.Sys().ListPolicies()
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	for _, name := range policies {
		policy, err := c.client.Sys().GetPolicy(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get policy %s: %w", name, err)
		}
		state[fmt.Sprintf("sys/policy/%s", name)] = map[string]interface{}{
			"policy": policy,
		}
	}

	// Get mounts
	mounts, err := c.client.Sys().ListMounts()
	if err != nil {
		return nil, fmt.Errorf("failed to list mounts: %w", err)
	}
	for path, mount := range mounts {
		state[fmt.Sprintf("sys/mounts/%s", path)] = map[string]interface{}{
			"type":        mount.Type,
			"description": mount.Description,
			"config":      mount.Config,
			"options":     mount.Options,
		}
	}

	return state, nil
}
