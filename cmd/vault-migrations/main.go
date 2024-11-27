package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/pampatzoglou/hashicorp-vault-migrations/pkg/migrations"
)

const usage = `vault-migrations - A tool for managing HashiCorp Vault configuration migrations

Usage:
  vault-migrations [flags]

Flags:
  -config string    Path to configuration file (default "config.yaml")
  -dry-run         Perform a dry run without making changes
  -log-level       Set logging level (debug, info, warn, error)
  -help            Show this help message
  -version         Show version information

Configuration File (YAML):
  vault:
    address: "http://vault:8200"        # Vault server address
    token: "${VAULT_TOKEN}"            # Vault token or use environment variable
    auth_method: "token"               # Authentication method (token, approle, kubernetes)
    role: "my-role"                    # Role for auth methods that require it
    namespace: "my-namespace"          # Optional Vault namespace
    max_retries: 3                     # Maximum number of retry attempts
    retry_delay: "1s"                  # Delay between retries

  migrations:
    directory: "./migrations"          # Directory containing migration files
    concurrent_tasks: true            # Run tasks concurrently within migrations
    stop_on_error: true              # Stop on first error

  log_level: "info"                   # Logging level
  dry_run: false                     # Perform dry run without making changes

Environment Variables:
  VAULT_ADDR      Alternative to config file vault.address
  VAULT_TOKEN     Alternative to config file vault.token
  VAULT_NAMESPACE Alternative to config file vault.namespace

Examples:
  # Run migrations with default config file
  vault-migrations

  # Run migrations with custom config file
  vault-migrations -config=/path/to/config.yaml

  # Perform a dry run with debug logging
  vault-migrations -dry-run -log-level=debug

Version: %s
`

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func printVersion() {
	fmt.Printf("vault-migrations version %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("build date: %s\n", date)
}

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without making changes")
	logLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version information")
	
	// Override default usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, version)
	}
	
	flag.Parse()

	// Show version if requested
	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Load configuration
	config, err := migrations.LoadConfig(*configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Override config with command line flags
	if *dryRun {
		config.DryRun = true
	}
	if *logLevel != "" {
		config.LogLevel = *logLevel
	}

	// Set log level
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid log level, defaulting to info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("Received signal, initiating graceful shutdown")
		cancel()
	}()

	// Initialize Vault client
	client, err := migrations.NewVaultClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Vault client")
	}

	// Create a MigrationRunner
	runner, err := migrations.NewMigrationRunner(client, config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MigrationRunner")
	}

	// Run the migrations
	if err := runner.RunMigrations(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	log.Info().Msg("Migrations completed successfully")
}
