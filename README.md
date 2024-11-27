# HashiCorp Vault Migrations

This tool helps manage and apply schema-driven migrations for HashiCorp Vault.

## Features
- Define desired Vault state in YAML (`schema.yaml`).
- Automatically generate migration files.
- Run migrations and track their status.

## Usage
1. Define the desired state in `schema.yaml`.
2. Generate migration files:
   ```bash
   go run cmd/vault-migrations/main.go --generate```
