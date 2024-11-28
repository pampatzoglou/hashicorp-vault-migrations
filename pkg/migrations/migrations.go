package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/hashicorp/vault/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	logger        zerolog.Logger
	dryRun        bool
}

// NewMigrationRunner initializes a new MigrationRunner.
func NewMigrationRunner(client *api.Client, config *Config) (*MigrationRunner, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if config.Migrations.Directory == "" {
		return nil, errors.New("migrations directory is required")
	}

	// For non-generate commands, we need a Vault client
	if client == nil && !config.DryRun {
		return nil, errors.New("Vault client is required for non-dry-run operations")
	}

	logger := log.With().Str("component", "migration-runner").Logger()

	return &MigrationRunner{
		client:        client,
		migrationsDir: config.Migrations.Directory,
		trackingPath:  "migrations/version",
		logger:        logger,
		dryRun:        config.DryRun,
	}, nil
}

// getLastAppliedVersion retrieves the last applied migration version.
func (m *MigrationRunner) getLastAppliedVersion(ctx context.Context) (int, error) {
	if m.client == nil {
		return 0, nil
	}

	secret, err := m.client.Logical().ReadWithContext(ctx, m.trackingPath)
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
func (m *MigrationRunner) setLastAppliedVersion(ctx context.Context, version int) error {
	if m.client == nil {
		return nil
	}

	data := map[string]interface{}{
		"version": strconv.Itoa(version),
	}
	_, err := m.client.Logical().WriteWithContext(ctx, m.trackingPath, data)
	if err != nil {
		return fmt.Errorf("failed to update tracking path: %w", err)
	}
	return nil
}

// loadMigrations loads migration files from the directory and sorts them by version.
func (m *MigrationRunner) loadMigrations(ctx context.Context) ([]Migration, error) {
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

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration.
func (m *MigrationRunner) applyMigration(ctx context.Context, migration Migration) error {
	if m.client == nil {
		return fmt.Errorf("cannot apply migration without Vault client")
	}

	m.logger.Info().Int("version", migration.Version).Msg("Applying migration")

	if m.dryRun {
		m.logger.Info().Int("version", migration.Version).Msg("Dry run - skipping migration")
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(migration.Tasks))

	for _, task := range migration.Tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			if err := m.executeTask(ctx, t); err != nil {
				errChan <- fmt.Errorf("failed to execute task: %w", err)
			}
		}(task)
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// executeTask executes a single Vault task
func (m *MigrationRunner) executeTask(ctx context.Context, task Task) error {
	if m.client == nil {
		return fmt.Errorf("cannot execute task without Vault client")
	}

	m.logger.Debug().
		Str("path", task.Path).
		Str("method", task.Method).
		Interface("data", task.Data).
		Msg("Executing task")

	switch task.Method {
	case "POST":
		_, err := m.client.Logical().WriteWithContext(ctx, task.Path, task.Data)
		return err
	case "PUT":
		_, err := m.client.Logical().WriteWithContext(ctx, task.Path, task.Data)
		return err
	case "DELETE":
		_, err := m.client.Logical().DeleteWithContext(ctx, task.Path)
		return err
	default:
		return fmt.Errorf("unsupported method: %s", task.Method)
	}
}

// RunMigrations executes all pending migrations.
func (m *MigrationRunner) RunMigrations(ctx context.Context) error {
	if m.client == nil {
		return fmt.Errorf("cannot run migrations without Vault client")
	}

	// Load all migrations
	migrations, err := m.loadMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get last applied version
	lastApplied, err := m.getLastAppliedVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last applied version: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if migration.Version <= lastApplied {
			continue
		}

		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		if !m.dryRun {
			if err := m.setLastAppliedVersion(ctx, migration.Version); err != nil {
				return fmt.Errorf("failed to update version after migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}
