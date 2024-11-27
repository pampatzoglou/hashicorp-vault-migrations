Architecture of HashiCorp Vault Migrations Tool

This document provides an overview of the architecture for the HashiCorp Vault Migrations tool. The system enables schema migrations for HashiCorp Vault, akin to database migrations. It supports defining, applying, and tracking migration tasks in YAML format and integrates with Vault's API for seamless operations.

Key Components

1. Command-Line Interface (CLI)

Location: cmd/vault-migrations/main.go

Purpose: Entry point for the tool, supporting commands such as:

--generate: Generates migration files based on defined changes.

--apply: Applies pending migrations.

Responsibilities:

Loads configuration from config.yaml.

Initializes Vault client and migration runner.

Handles user input and commands.

2. Configuration Loader

Location: pkg/migrations/config.go

Purpose: Reads and parses the config.yaml file.

Key Features:

Supports specifying the Vault address and token file.

Defines the migrations directory.

3. Vault Client

Location: pkg/migrations/client.go

Purpose: Manages the connection to the Vault server using the HashiCorp Vault Go API.

Key Features:

Reads token from a file for authentication.

Supports API calls for reading, writing, and managing secrets.

4. Migration Runner

Location: pkg/migrations/migrations.go

Purpose: Core component responsible for loading, applying, and tracking migrations.

Key Features:

Load Migrations: Reads YAML files from the specified migrations directory.

Track State: Uses a special Vault path (e.g., migrations/version) to store the last applied migration.

Apply Migrations: Executes tasks (e.g., write, delete) sequentially, ensuring order and consistency.

5. Migration Generator

Location: pkg/migrations/generator.go

Purpose: Automatically generates migration files based on desired state changes.

Key Features:

Compares current Vault state with desired state definitions.

Suggests changes and outputs them as migration YAML files.

Data Flow

Load Configuration:

The CLI loads config.yaml using the configuration loader.

Vault token is read from the specified token file.

Initialize Vault Client:

Establishes a connection to the Vault server using the provided address and token.

Load and Apply Migrations:

The MigrationRunner loads all .yaml files from the migrations directory.

Sorts migrations by version.

Checks the migrations/version path in Vault to determine the last applied version.

Applies migrations sequentially if they have not already been applied.

Generate Migrations:

The generator compares the desired state definitions with the current Vault state.

Outputs YAML files with necessary changes.

Architecture Diagram

The following diagram represents the high-level architecture:



Key Components:

CLI interacts with the user and drives the workflow.

Configuration Loader provides runtime configurations.

Vault Client communicates with the Vault server.

Migration Runner handles the execution and tracking of migrations.

Migration Generator automates migration file creation.

Sequence Diagram

Applying Migrations

```sequence {theme="hand"}

    User->>CLI: Run --apply command
    CLI->>ConfigLoader: Load config.yaml
    ConfigLoader-->>CLI: Return configuration
    CLI->>VaultClient: Initialize connection
    VaultClient-->>CLI: Connection established
    CLI->>MigrationRunner: Load migrations
    MigrationRunner->>VaultClient: Fetch last applied version
    VaultClient-->>MigrationRunner: Return version
    MigrationRunner->>VaultServer: Apply pending migrations
    VaultServer-->>MigrationRunner: Confirm success
    MigrationRunner-->>CLI: Migration applied
    CLI-->>User: Output results
```
Generating Migrations

```sequence {theme="hand"}
    participant User
    participant CLI
    participant ConfigLoader
    participant VaultClient
    participant MigrationGenerator

    User->>CLI: Run --generate command
    CLI->>ConfigLoader: Load config.yaml
    ConfigLoader-->>CLI: Return configuration
    CLI->>VaultClient: Initialize connection
    VaultClient-->>CLI: Connection established
    CLI->>MigrationGenerator: Compare desired and current state
    MigrationGenerator->>VaultClient: Fetch current state
    VaultClient-->>MigrationGenerator: Return current state
    MigrationGenerator-->>CLI: Output migration files
    CLI-->>User: Confirm generation success
```
Future Improvements

Enhanced Error Reporting:

Detailed logs for failed tasks.

Retry mechanisms for transient failures.

Metrics Tracking:

Time taken for each migration.

Number of migrations applied.

State Management:

Version control for migrations.

Rollback support for failed migrations.

Testing Framework:

Unit and integration tests for migrations.

Mock Vault server for local testing.
