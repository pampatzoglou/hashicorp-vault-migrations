package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// createTestMigrationFile creates a test migration file with the given version and tasks
func createTestMigrationFile(t *testing.T, dir string, version int, tasks []Task) string {
	filename := fmt.Sprintf("%03d_test.yaml", version)
	path := filepath.Join(dir, filename)

	migration := Migration{
		Version: version,
		Tasks:   tasks,
	}

	data, err := yaml.Marshal(migration)
	require.NoError(t, err)

	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)

	return path
}

// createTestConfig creates a test configuration file
func createTestConfig(t *testing.T, dir string, config *Config) string {
	path := filepath.Join(dir, "config.yaml")

	data, err := yaml.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)

	return path
}

// createTempDir creates a temporary directory and returns its path
// The directory will be automatically cleaned up when the test completes
func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "vault-migrations-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// withTestConfig runs a test with a temporary configuration
func withTestConfig(t *testing.T, config *Config, fn func(configPath string)) {
	dir := createTempDir(t)
	configPath := createTestConfig(t, dir, config)
	fn(configPath)
}

// withTestMigrations runs a test with temporary migration files
func withTestMigrations(t *testing.T, migrations []Migration, fn func(migrationsDir string)) {
	dir := createTempDir(t)
	for _, migration := range migrations {
		createTestMigrationFile(t, dir, migration.Version, migration.Tasks)
	}
	fn(dir)
}
