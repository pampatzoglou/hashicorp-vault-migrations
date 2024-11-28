package migrations

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"
)

// HCLDiff represents a difference in HCL configuration
type HCLDiff struct {
	Path     string
	OldValue interface{}
	NewValue interface{}
}

// StateFile represents the last known state
type StateFile struct {
	LastKnownState map[string]interface{} `yaml:"last_known_state"`
}

// getLastKnownState retrieves the last known state from the state file
func getLastKnownState(migrationsDir string) (map[string]interface{}, error) {
	statePath := filepath.Join(migrationsDir, ".state.yaml")
	
	// If state file doesn't exist, return empty state
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return state.LastKnownState, nil
}

// saveLastKnownState saves the current state to the state file
func saveLastKnownState(migrationsDir string, state map[string]interface{}) error {
	stateFile := StateFile{
		LastKnownState: state,
	}

	data, err := yaml.Marshal(stateFile)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := filepath.Join(migrationsDir, ".state.yaml")
	if err := ioutil.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GenerateMigration creates a new migration file.
func GenerateMigration(version int, tasks []Task, outputDir string) error {
	migration := Migration{
		Version: version,
		Tasks:   tasks,
	}

	filename := fmt.Sprintf("%s.yaml", sanitizeFilename(fmt.Sprintf("migration_%d", version)))
	outputPath := filepath.Join(outputDir, filename)

	data, err := yaml.Marshal(migration)
	if err != nil {
		return fmt.Errorf("failed to marshal migration: %w", err)
	}

	if err := ioutil.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	return nil
}

// GenerateIntelligentMigration generates a migration based on the current state and desired configuration
func GenerateIntelligentMigration(currentConfig, desiredConfig map[string]interface{}, migrationsDir string) (string, error) {
	// Get the latest version number
	version, err := getLatestVersion(migrationsDir)
	if err != nil {
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}

	// Get last known state if no current config is provided
	if currentConfig == nil {
		lastKnownState, err := getLastKnownState(migrationsDir)
		if err != nil {
			return "", fmt.Errorf("failed to get last known state: %w", err)
		}
		currentConfig = lastKnownState
	}

	// If no current config and no last known state, generate a full migration
	if currentConfig == nil {
		if len(desiredConfig) == 0 {
			return "No migrations required - empty desired state", nil
		}

		// Create tasks for all desired configurations
		var tasks []Task
		for path, value := range desiredConfig {
			task := Task{
				Path:   path,
				Method: "POST",
				Data:   toMapStringInterface(value),
			}
			tasks = append(tasks, task)
		}

		// Generate the migration file
		if err := GenerateMigration(version+1, tasks, migrationsDir); err != nil {
			return "", fmt.Errorf("failed to generate migration: %w", err)
		}

		// Save the new state
		if err := saveLastKnownState(migrationsDir, desiredConfig); err != nil {
			return "", fmt.Errorf("failed to save state: %w", err)
		}

		return fmt.Sprintf("Generated initial migration version %d with %d tasks", version+1, len(tasks)), nil
	}

	// Compare configurations and get differences
	diffs := compareConfigs(currentConfig, desiredConfig)
	if len(diffs) == 0 {
		return "No migrations required - configurations are identical", nil
	}

	// Create tasks from differences
	tasks := generateTasksFromDiffs(diffs)
	if len(tasks) == 0 {
		return "No migrations required - no actionable differences found", nil
	}

	// Generate the migration file
	if err := GenerateMigration(version+1, tasks, migrationsDir); err != nil {
		return "", fmt.Errorf("failed to generate migration: %w", err)
	}

	// Save the new state
	if err := saveLastKnownState(migrationsDir, desiredConfig); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	return fmt.Sprintf("Generated migration version %d with %d tasks", version+1, len(tasks)), nil
}

// getLatestVersion gets the latest migration version from the migrations directory
func getLatestVersion(migrationsDir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.yaml"))
	if err != nil {
		return 0, err
	}

	if len(files) == 0 {
		return 0, nil
	}

	versions := make([]int, 0, len(files))
	for _, file := range files {
		var migration Migration
		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &migration); err != nil {
			continue
		}
		versions = append(versions, migration.Version)
	}

	if len(versions) == 0 {
		return 0, nil
	}

	sort.Ints(versions)
	return versions[len(versions)-1], nil
}

// compareConfigs compares two configurations and returns the differences
func compareConfigs(current, desired map[string]interface{}) []HCLDiff {
	var diffs []HCLDiff

	// Compare desired against current
	for path, desiredValue := range desired {
		currentValue, exists := current[path]
		if !exists {
			// New configuration
			diffs = append(diffs, HCLDiff{
				Path:     path,
				NewValue: desiredValue,
			})
			continue
		}

		if !configValuesEqual(currentValue, desiredValue) {
			// Changed configuration
			diffs = append(diffs, HCLDiff{
				Path:     path,
				OldValue: currentValue,
				NewValue: desiredValue,
			})
		}
	}

	// Check for removed configurations
	for path, currentValue := range current {
		if _, exists := desired[path]; !exists {
			// Removed configuration
			diffs = append(diffs, HCLDiff{
				Path:     path,
				OldValue: currentValue,
			})
		}
	}

	return diffs
}

// configValuesEqual compares two configuration values for equality
func configValuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr
}

// generateTasksFromDiffs converts HCL differences into Vault tasks
func generateTasksFromDiffs(diffs []HCLDiff) []Task {
	var tasks []Task

	for _, diff := range diffs {
		// Skip if both old and new values are nil
		if diff.OldValue == nil && diff.NewValue == nil {
			continue
		}

		method := "POST"
		if diff.OldValue != nil && diff.NewValue == nil {
			method = "DELETE"
		} else if diff.OldValue != nil {
			method = "PUT"
		}

		task := Task{
			Path:   diff.Path,
			Method: method,
		}

		if diff.NewValue != nil {
			task.Data = toMapStringInterface(diff.NewValue)
		}

		tasks = append(tasks, task)
	}

	return tasks
}

// toMapStringInterface converts an interface{} to map[string]interface{}
func toMapStringInterface(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{"value": v}
}
