package main

import (
	"fmt"
	"log"

	"github.com/pampatzoglou/hashicorp-vault-migrations/pkg/migrations"
)

func main() {
	// Load configuration
	config, err := migrations.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Vault client
	client, err := migrations.NewVaultClient(config)
	if err != nil {
		log.Fatalf("Failed to initialize Vault client: %v", err)
	}

	// Create a MigrationRunner
	runner, err := migrations.NewMigrationRunner(client, config)
	if err != nil {
		log.Fatalf("Failed to create MigrationRunner: %v", err)
	}

	// Run the migrations
	if err := runner.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Migrations applied successfully.")
}
