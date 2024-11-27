package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/pampatzoglou/hashicorp-vault-migrations/pkg/migrations"
	"gopkg.in/yaml.v3"
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
  -generate        Generate a new migration file with a template

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
	generateMigration := flag.Bool("generate", false, "Generate a new migration file with a template")
	
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

	// Handle migration generation if requested
	if *generateMigration {
		if err := handleMigrationGeneration(*configFile); err != nil {
			log.Fatal().Err(err).Msg("Failed to generate migration")
		}
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

// Schema represents the desired state from schema.yaml
type Schema struct {
	Paths []struct {
		Path   string                 `yaml:"path"`
		Method string                 `yaml:"method"`
		Data   map[string]interface{} `yaml:"data"`
	} `yaml:"paths"`
}

// Task represents a single migration task
type Task struct {
	Path   string                 `yaml:"path"`
	Method string                 `yaml:"method"`
	Data   map[string]interface{} `yaml:"data,omitempty"`
}

// Migration represents a migration file
type Migration struct {
	Version int    `yaml:"version"`
	Tasks   []Task `yaml:"tasks"`
}

// PathConfig represents the configuration for different types of Vault paths
type PathConfig struct {
	DataWrapper bool     // Whether to wrap data in a "data" key
	Methods     []string // Valid methods for this path type
}

// getPathConfig returns the configuration for different Vault path types
func getPathConfig(path string) PathConfig {
	switch {
	case strings.HasPrefix(path, "secret/data/"):
		return PathConfig{
			DataWrapper: true,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	case strings.HasPrefix(path, "auth/token/roles/"):
		return PathConfig{
			DataWrapper: false,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	case strings.HasPrefix(path, "sys/auth/"):
		return PathConfig{
			DataWrapper: false,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	case strings.HasPrefix(path, "sys/mounts/"):
		return PathConfig{
			DataWrapper: false,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	case strings.HasPrefix(path, "sys/policies/"):
		return PathConfig{
			DataWrapper: false,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	default:
		return PathConfig{
			DataWrapper: false,
			Methods:    []string{"read", "write", "delete", "list"},
		}
	}
}

// handleMigrationGeneration handles the generation of a new migration file
func handleMigrationGeneration(configFile string) error {
	// Load configuration but skip validation for generate command
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config migrations.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// If migrations directory is not specified, use default
	if config.Migrations.Directory == "" {
		config.Migrations.Directory = "migrations"
	}

	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll(config.Migrations.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Load schema.yaml
	schemaData, err := os.ReadFile("schema.yaml")
	if err != nil {
		return fmt.Errorf("failed to read schema.yaml: %w", err)
	}

	var schema Schema
	if err := yaml.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("failed to parse schema.yaml: %w", err)
	}

	// Get current state from existing migrations
	currentState, err := getCurrentState(config.Migrations.Directory)
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Generate new tasks based on the difference between schema and current state
	newTasks := generateNewTasks(schema.Paths, currentState)
	if len(newTasks) == 0 {
		fmt.Println("No new changes detected in schema.yaml")
		return nil
	}

	// Get the next version number
	nextVersion, err := getNextMigrationVersion(config.Migrations.Directory)
	if err != nil {
		return fmt.Errorf("failed to determine next migration version: %w", err)
	}

	// Generate the migration file
	filename := fmt.Sprintf("migration_%03d.yaml", nextVersion)
	filepath := filepath.Join(config.Migrations.Directory, filename)

	migration := Migration{
		Version: nextVersion,
		Tasks:   newTasks,
	}

	data, err = yaml.Marshal(migration)
	if err != nil {
		return fmt.Errorf("failed to marshal migration: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	fmt.Printf("Generated new migration file: %s\n", filepath)
	return nil
}

// getCurrentState reads all existing migrations and builds a map of the current state
func getCurrentState(migrationsDir string) (map[string]Task, error) {
	state := make(map[string]Task)
	
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.yaml"))
	if err != nil {
		return state, nil
	}

	// Sort files to process them in order
	sort.Strings(files)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var migration Migration
		if err := yaml.Unmarshal(data, &migration); err != nil {
			continue
		}

		// Update state with each task
		for _, task := range migration.Tasks {
			// For write operations, update the state
			if task.Method == "write" {
				state[task.Path] = task
			} else if task.Method == "delete" {
				// For delete operations, remove from state
				delete(state, task.Path)
			}
		}
	}

	return state, nil
}

// generateNewTasks compares desired state with current state and generates necessary tasks
func generateNewTasks(desiredPaths []struct {
	Path   string                 `yaml:"path"`
	Method string                 `yaml:"method"`
	Data   map[string]interface{} `yaml:"data"`
}, currentState map[string]Task) []Task {
	var newTasks []Task

	// Helper function to compare data
	dataChanged := func(current, desired map[string]interface{}) bool {
		// Handle the case where current state has data wrapper
		currentData := current
		if wrapped, ok := current["data"].(map[string]interface{}); ok {
			currentData = wrapped
		}
		return !reflect.DeepEqual(currentData, desired)
	}

	for _, desired := range desiredPaths {
		pathConfig := getPathConfig(desired.Path)
		
		// Split methods and validate them
		methods := strings.Split(desired.Method, ",")
		validMethods := make([]string, 0)
		for _, method := range methods {
			method = strings.TrimSpace(method)
			// Validate method against allowed methods for this path type
			for _, allowedMethod := range pathConfig.Methods {
				if method == allowedMethod {
					validMethods = append(validMethods, method)
					break
				}
			}
		}

		current, exists := currentState[desired.Path]
		currentMethods := make(map[string]bool)
		if exists {
			for _, m := range strings.Split(current.Method, ",") {
				currentMethods[strings.TrimSpace(m)] = true
			}
		}

		// Check if we need to generate tasks
		needsUpdate := false
		if !exists {
			needsUpdate = true
		} else {
			// Check if methods have changed
			currentMethodSet := make(map[string]bool)
			for _, m := range strings.Split(current.Method, ",") {
				currentMethodSet[strings.TrimSpace(m)] = true
			}
			desiredMethodSet := make(map[string]bool)
			for _, m := range validMethods {
				desiredMethodSet[m] = true
			}
			
			if !reflect.DeepEqual(currentMethodSet, desiredMethodSet) {
				needsUpdate = true
			}

			// Check if data has changed for write operations
			if desiredMethodSet["write"] && dataChanged(current.Data, desired.Data) {
				needsUpdate = true
			}
		}

		if needsUpdate {
			// Generate tasks for all methods in the schema
			for _, method := range validMethods {
				task := Task{
					Path:   desired.Path,
					Method: method,
				}

				// Only include data for write operations
				if method == "write" {
					if pathConfig.DataWrapper {
						task.Data = map[string]interface{}{
							"data": desired.Data,
						}
					} else {
						task.Data = desired.Data
					}
				}

				newTasks = append(newTasks, task)
			}
		}
	}

	// Check for complete path deletions
	desiredPathsMap := make(map[string]bool)
	for _, p := range desiredPaths {
		desiredPathsMap[p.Path] = true
	}

	for path := range currentState {
		if !desiredPathsMap[path] {
			newTasks = append(newTasks, Task{
				Path:   path,
				Method: "delete",
			})
		}
	}

	return newTasks
}

// getNextMigrationVersion determines the next migration version by scanning the migrations directory
func getNextMigrationVersion(migrationsDir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.yaml"))
	if err != nil {
		return 1, nil // Start with version 1 if there's an error or no files
	}

	if len(files) == 0 {
		return 1, nil // Start with version 1 if no files exist
	}

	maxVersion := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var migration Migration
		if err := yaml.Unmarshal(data, &migration); err != nil {
			continue
		}

		if migration.Version > maxVersion {
			maxVersion = migration.Version
		}
	}

	return maxVersion + 1, nil
}
