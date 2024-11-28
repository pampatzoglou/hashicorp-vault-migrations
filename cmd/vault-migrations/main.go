package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/pampatzoglou/hashicorp-vault-migrations/pkg/migrations"
)

const usage = `vault-migrations - A tool for managing HashiCorp Vault configuration migrations

Usage:
  vault-migrations [flags]

Flags:
  --config string     Path to configuration file (default "config.yaml")
  --schema string     Path to schema file (default "schema.yaml")
  --dry-run          Perform a dry run without making changes
  --log-level        Set logging level (debug, info, warn, error)
  --generate         Generate migration from schema
  --help             Show this help message
  --version          Show version information

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

  # Run migrations with custom config and schema files
  vault-migrations --config=/path/to/config.yaml --schema=/path/to/schema.yaml

  # Generate migration from schema
  vault-migrations --generate --schema=/path/to/schema.yaml

  # Perform a dry run with debug logging
  vault-migrations --dry-run --log-level=debug

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

// normalizeFlag converts a flag to use -- prefix if needed
func normalizeFlag(name string) string {
	if !strings.HasPrefix(name, "--") {
		return "--" + name
	}
	return name
}

func main() {
	// Create custom FlagSet to handle -- prefix
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, version)
	}

	// Parse command line flags
	configFile := fs.String("config", "config.yaml", "Path to configuration file")
	schemaFile := fs.String("schema", "schema.yaml", "Path to schema file")
	dryRun := fs.Bool("dry-run", false, "Perform a dry run without making changes")
	logLevel := fs.String("log-level", "", "Log level (debug, info, warn, error)")
	showVersion := fs.Bool("version", false, "Show version information")
	generate := fs.Bool("generate", false, "Generate migration from schema")

	// Handle -- prefix for flags
	args := make([]string, 0, len(os.Args[1:]))
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			arg = "-" + arg
		}
		args = append(args, arg)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Show version if requested
	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	if *logLevel != "" {
		level, err := zerolog.ParseLevel(*logLevel)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid log level")
		}
		zerolog.SetGlobalLevel(level)
	}

	// Load configuration
	var config *migrations.Config
	var err error
	if *configFile != "" {
		config, err = migrations.LoadConfig(*configFile)
		if err != nil && !*generate {
			log.Fatal().Err(err).Msg("failed to load configuration")
		} else if err != nil {
			// For generate command, we'll create a minimal config
			config = &migrations.Config{
				Migrations: migrations.MigrationsConfig{
					Directory: "./migrations", // default directory
				},
			}
		}

		// Validate configuration based on mode
		if err := config.Validate(*generate); err != nil {
			log.Fatal().Err(err).Msg("invalid configuration")
		}
	}

	// Override dry-run from command line if specified or in generate mode
	if *dryRun || *generate {
		if config != nil {
			config.DryRun = true
		}
	}

	// Create migration runner
	var runner *migrations.MigrationRunner
	if config != nil {
		// For non-generate commands, create a Vault client
		var client *api.Client
		var err error
		
		if !*generate {
			vaultClient, err := migrations.NewVaultClient(config.Vault)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create Vault client")
			}
			client = vaultClient.GetClient()
		}

		runner, err = migrations.NewMigrationRunner(client, config)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create migration runner")
		}
	}

	// Handle generate command
	if *generate {
		migrationsDir := "./migrations" // default directory
		if config != nil && config.Migrations.Directory != "" {
			migrationsDir = config.Migrations.Directory
		}

		var currentConfig map[string]interface{}
		var err error

		// Try to connect to Vault and get current state
		if config != nil && config.Vault.Address != "" {
			client, err := migrations.NewVaultClient(config.Vault)
			if err == nil {
				currentConfig, err = client.GetCurrentState()
				if err != nil {
					log.Warn().Err(err).Msg("failed to get current state from Vault, will generate migration from schema only")
				}
			} else {
				log.Warn().Err(err).Msg("failed to connect to Vault, will generate migration from schema only")
			}
		}

		// Generate migration based on schema and available state
		schema, err := migrations.LoadSchema(*schemaFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load schema")
		}
		result, err := migrations.GenerateIntelligentMigration(currentConfig, schema.DesiredState, migrationsDir)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate migration")
		}
		log.Info().Msg(result)
		return
	}

	// For non-generate commands, we need a valid config
	if config == nil {
		log.Fatal().Msg("configuration is required for non-generate commands")
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info().Msgf("received signal %s, initiating shutdown", sig)
		cancel()
	}()

	// Run migrations
	if err := runner.RunMigrations(ctx); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
}
