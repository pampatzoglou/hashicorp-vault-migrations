package migrations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
)

// Task defines a single Vault operation.
type Task struct {
	Path   string                 `yaml:"path"`
	Method string                 `yaml:"method"`
	Data   map[string]interface{} `yaml:"data"`
}

// Migration groups a set of tasks into a migration file.
type Migration struct {
	Version int    `yaml:"version"`
	Tasks   []Task `yaml:"tasks"`
}

// MigrationRunner handles running and tracking migrations.
type MigrationRunner struct {
	client        *api.Client
	migrationsDir string
	trackingPath  string
}

// NewMigrationRunner initializes a new MigrationRunner.
func NewMigrationRunner(client *api.Client, config *Config) (*MigrationRunner, error) {
	if client == nil {
		return nil, errors.New("Vault client is required")
	}
	if config.Migrations.Directory == "" {
		return nil, errors.New("migrations directory is required")
	}

	return &MigrationRunner{
		client:        client,
		migrationsDir: config.Migrations.Directory,
		trackingPath:  "migrations/version", // Vault path for tracking migration state
	}, nil
}

// getLastAppliedVersion retrieves the last applied migration version.
func (m *MigrationRunner) getLastAppliedVersion() (int, error) {
	secret, err := m.client.Logical().Read(m.trackingPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read tracking path: %w", err)
	}
	if secret == nil || secret.Data["version"] == nil {
		return 0, nil
	}

	version, err := strconv.Atoi(secret.Data["version"].(string))
	if err != nil {
		return 0, fmt.Errorf("invalid version format: %w", err)
	}
	return version, nil
}

// setLastAppliedVersion updates the last applied migration version in Vault.
func (m *MigrationRunner) setLastAppliedVersion(version int) error {
	data := map[string]interface{}{
		"version": strconv.Itoa(version),
	}
	_, err := m.client.Logical().Write(m.trackingPath, data)
	if err != nil {
		return fmt.Errorf("failed to update tracking path: %w", err)
	}
	return nil
}

// loadMigrations loads migration files from the directory and sorts them by version.
func (m *MigrationRunner) loadMigrations() ([]Migration, error) {
	files, err := filepath.Glob(filepath.Join(m.migrationsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read migration files: %w", err)
	}

	var migrations []Migration
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		var migration Migration
		if err := yaml.Unmarshal(data, &migration); err != nil {
			return nil, fmt.Errorf("failed to parse migration file %s: %w", file, err)
		}

		migrations = append(migrations, migration)
	}

	// Sort migrations by version.
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration.
func (m *MigrationRunner) applyMigration(migration Migration) error {
	for _, task := range migration.Tasks {
		var err error
		switch task.Method {
		case "write":
			_, err = m.client.Logical().Write(task.Path, task.Data)
		case "delete":
			_, err = m.client.Logical().Delete(task.Path)
		default:
			return fmt.Errorf("unsupported method %s for path %s", task.Method, task.Path)
		}

		if err != nil {
			return fmt.Errorf("failed to apply task at path %s: %w", task.Path, err)
		}
	}
	return nil
}

// RunMigrations executes all pending migrations.
func (m *MigrationRunner) RunMigrations() error {
	startTime := time.Now()

	lastVersion, err := m.getLastAppliedVersion()
	if err != nil {
		return fmt.Errorf("failed to get last applied version: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	for _, migration := range migrations {
		if migration.Version > lastVersion {
			fmt.Printf("Applying migration version %d...\n", migration.Version)
			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration version %d: %w", migration.Version, err)
			}

			if err := m.setLastAppliedVersion(migration.Version); err != nil {
				return fmt.Errorf("failed to update last applied version: %w", err)
			}

			fmt.Printf("Migration version %d applied successfully.\n", migration.Version)
		}
	}

	fmt.Printf("Migrations completed in %s.\n", time.Since(startTime))
	return nil
}
