package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationRunner_LoadMigrations(t *testing.T) {
	// Create temporary directory for test migrations
	tmpDir, err := os.MkdirTemp("", "vault-migrations-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test migration files
	testMigrations := []struct {
		filename string
		content  string
	}{
		{
			"001_init.yaml",
			`version: 1
tasks:
  - path: secret/data/test
    method: write
    data:
      data:
        key: value`,
		},
		{
			"002_update.yaml",
			`version: 2
tasks:
  - path: secret/data/test
    method: write
    data:
      data:
        key2: value2`,
		},
	}

	for _, tm := range testMigrations {
		err := os.WriteFile(
			filepath.Join(tmpDir, tm.filename),
			[]byte(tm.content),
			0644,
		)
		require.NoError(t, err)
	}

	// Create test runner
	runner := &MigrationRunner{
		migrationsDir: tmpDir,
	}

	// Test loading migrations
	migrations, err := runner.loadMigrations(context.Background())
	require.NoError(t, err)
	assert.Len(t, migrations, 2)
	assert.Equal(t, 1, migrations[0].Version)
	assert.Equal(t, 2, migrations[1].Version)
}

func TestMigrationRunner_ApplyMigration(t *testing.T) {
	// Create test runner
	runner := &MigrationRunner{
		client: &api.Client{},
	}

	// Create test migration
	migration := Migration{
		Version: 1,
		Tasks: []Task{
			{
				Path:   "secret/data/test",
				Method: "write",
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"key": "value",
					},
				},
			},
		},
	}

	// Test applying migration
	ctx := context.Background()
	err := runner.applyMigration(ctx, migration)
	require.NoError(t, err)
}

func TestMigrationRunner_VersionTracking(t *testing.T) {
	// Create test runner
	runner := &MigrationRunner{
		client:       &api.Client{},
		trackingPath: "migrations/version",
	}

	// Test version tracking
	ctx := context.Background()
	version := 123

	err := runner.setLastAppliedVersion(ctx, version)
	require.NoError(t, err)

	lastVersion, err := runner.getLastAppliedVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, version, lastVersion)
}

func TestMigrationRunner_ConcurrentTasks(t *testing.T) {
	// Create test runner
	runner := &MigrationRunner{
		client: &api.Client{},
	}

	// Create test migration with multiple tasks
	migration := Migration{
		Version: 1,
		Tasks: []Task{
			{
				Path:   "secret/data/test1",
				Method: "write",
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"key1": "value1",
					},
				},
			},
			{
				Path:   "secret/data/test2",
				Method: "write",
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"key2": "value2",
					},
				},
			},
		},
	}

	// Test concurrent execution
	ctx := context.Background()
	start := time.Now()
	err := runner.applyMigration(ctx, migration)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, duration, 2*time.Second) // Tasks should run concurrently
}

func TestMigrationRunner_DryRun(t *testing.T) {
	// Create test runner with dry run enabled
	runner := &MigrationRunner{
		client: &api.Client{},
		dryRun: true,
	}

	// Create test migration
	migration := Migration{
		Version: 1,
		Tasks: []Task{
			{
				Path:   "secret/data/test",
				Method: "write",
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"key": "value",
					},
				},
			},
		},
	}

	// Test dry run
	ctx := context.Background()
	err := runner.applyMigration(ctx, migration)
	require.NoError(t, err)
	// In dry run mode, no actual changes should be made to Vault
}
