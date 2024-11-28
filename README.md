# Vault Migrations Tool

A tool for managing HashiCorp Vault configuration changes across different environments using a migration-based approach.

## Features

- Migration-based configuration management
- Intelligent state tracking
- Support for multiple secret engines (KV, PKI, AppRole, PostgreSQL)
- Secure binary signing
- Kubernetes-ready deployment

## Quick Start

1. Define the desired state in `schema.yaml`.
2. Generate migration files:
   ```bash
   go run cmd/vault-migrations/main.go --generate
   ```
3. Apply migrations:
   ```bash
   go run cmd/vault-migrations/main.go apply
   ```

## Build Container

1. For development:
   ```bash
   docker build --target development -t vault-migrations:dev .
   ```

2. For production without signing:
   ```bash
   docker build \
      --build-arg TAG=$(git describe --tags --always) \
      -t vault-migrations:latest .
   ```

3. For production with signing:
   ```bash
   # Generate cosign keypair if you haven't already
   cosign generate-key-pair

   # Build with signing
   DOCKER_BUILDKIT=1 docker build \
      --build-arg TAG=$(git describe --tags --always) \
      --build-arg SIGN_BINARY=true \
      --secret id=cosign_private_key,src=cosign.key \
      --secret id=cosign_public_key,src=cosign.pub \
      -t vault-migrations:signed .
   ```

## Deploy to Kind using Skaffold

1. Prerequisites:
   ```bash
   # Install Kind if not already installed
   curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
   chmod +x ./kind
   sudo mv ./kind /usr/local/bin/

   # Install Skaffold
   curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
   chmod +x skaffold
   sudo mv skaffold /usr/local/bin
   ```

2. Create Kind cluster:
   ```bash
   kind create cluster --name vault-migrations

   # Set kubectl context
   kubectl cluster-info --context kind-vault-migrations
   ```

3. Deploy using Skaffold:
   ```bash
   # Development mode with hot-reload
   skaffold dev

   # One-time deployment
   skaffold run
   ```

## Helm Chart

The Helm chart is located in `deploy/hashicorp-vault-migrations` and includes Vault server as an optional dependency.

### Installation

1. Install from local chart:
   ```bash
   helm install vault-migrations ./deploy/hashicorp-vault-migrations \
     --namespace vault-migrations \
     --create-namespace \
     --set vault.address=https://vault.example.com \
     --set vault.token=hvs.xxx
   ```

### Configuration

Key Helm chart parameters:

```yaml
# values.yaml
image:
  repository: hashicorp-vault-migrations
  tag: latest
  pullPolicy: IfNotPresent

vault:
  enabled: true  # Enable Vault server deployment
  address: ""    # External Vault address if not using embedded server
  token: ""
  namespace: ""
  skipVerify: false
  existingSecret: ""  # Use existing secret for Vault token
  server:
    dev:
      enabled: true   # Enable dev mode for embedded server
    standalone:
      enabled: false
    ha:
      enabled: false

config:
  logLevel: info
  logFormat: json
  schema:
    configMap: ""  # ConfigMap containing schema.yaml
    key: schema.yaml

cronJob:
  enabled: false
  schedule: "0 * * * *"
```

## Development

### Prerequisites

- Go 1.23.3 or later
- Docker
- Kind (for local Kubernetes testing)
- Skaffold
- Helm

### Project Structure

```
.
├── cmd/
│   └── vault-migrations/     # CLI application
├── pkg/
│   └── migrations/          # Core migration logic
├── deploy/
│   └── hashicorp-vault-migrations/  # Helm chart
├── migrations/             # Generated migrations
└── schema.yaml            # Desired state definition
```

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.