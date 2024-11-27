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
	"time"

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
	if client == nil {
		return nil, errors.New("Vault client is required")
	}
	if config.Migrations.Directory == "" {
		return nil, errors.New("migrations directory is required")
	}

	logger := log.With().Str("component", "migration-runner").Logger()

	return &MigrationRunner{
		client:        client,
		migrationsDir: config.Migrations.Directory,
		trackingPath:  "migrations/version",
		logger:        logger,
		dryRun:       config.DryRun,
	}, nil
}

// getLastAppliedVersion retrieves the last applied migration version.
func (m *MigrationRunner) getLastAppliedVersion(ctx context.Context) (int, error) {
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

	// Sort migrations by version.
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration.
func (m *MigrationRunner) applyMigration(ctx context.Context, migration Migration) error {
	m.logger.Info().Int("version", migration.Version).Msg("Applying migration")

	// Create error channel and wait group for concurrent task execution
	errChan := make(chan error, len(migration.Tasks))
	var wg sync.WaitGroup

	// Execute tasks concurrently
	for i, task := range migration.Tasks {
		wg.Add(1)
		go func(taskNum int, t Task) {
			defer wg.Done()

			logger := m.logger.With().
				Int("version", migration.Version).
				Int("task", taskNum).
				Str("path", t.Path).
				Str("method", t.Method).
				Logger()

			if m.dryRun {
				logger.Info().Msg("Dry run: would execute task")
				return
			}

			start := time.Now()
			var err error

			// Implement retries with backoff
			for retries := 0; retries < 3; retries++ {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
					if err = m.executeTask(ctx, t); err == nil {
						break
					}
					if retries < 2 {
						backoff := time.Duration(retries+1) * time.Second
						logger.Warn().Err(err).Int("retry", retries+1).Msg("Task failed, retrying")
						time.Sleep(backoff)
					}
				}
			}

			if err != nil {
				logger.Error().Err(err).Msg("Task failed after retries")
				errChan <- fmt.Errorf("task %d failed: %w", taskNum, err)
				return
			}

			logger.Info().
				Dur("duration", time.Since(start)).
				Msg("Task completed successfully")
		}(i, task)
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// executeTask executes a single Vault task
func (m *MigrationRunner) executeTask(ctx context.Context, task Task) error {
	var err error

	switch task.Method {
	case "read":
		_, err = m.client.Logical().ReadWithContext(ctx, task.Path)
	case "write":
		_, err = m.client.Logical().WriteWithContext(ctx, task.Path, task.Data)
	case "delete":
		_, err = m.client.Logical().DeleteWithContext(ctx, task.Path)
	default:
		return fmt.Errorf("unsupported method: %s", task.Method)
	}

	if err != nil {
		return fmt.Errorf("vault operation failed: %w", err)
	}

	return nil
}

// RunMigrations executes all pending migrations.
func (m *MigrationRunner) RunMigrations(ctx context.Context) error {
	m.logger.Info().Msg("Starting migrations")

	startTime := time.Now()

	lastVersion, err := m.getLastAppliedVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last applied version: %w", err)
	}

	migrations, err := m.loadMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	for _, migration := range migrations {
		if migration.Version > lastVersion {
			fmt.Printf("Applying migration version %d...\n", migration.Version)
			if err := m.applyMigration(ctx, migration); err != nil {
				return fmt.Errorf("failed to apply migration version %d: %w", migration.Version, err)
			}

			if err := m.setLastAppliedVersion(ctx, migration.Version); err != nil {
				return fmt.Errorf("failed to update last applied version: %w", err)
			}

			fmt.Printf("Migration version %d applied successfully.\n", migration.Version)
		}
	}

	fmt.Printf("Migrations completed in %s.\n", time.Since(startTime))
	return nil
}
