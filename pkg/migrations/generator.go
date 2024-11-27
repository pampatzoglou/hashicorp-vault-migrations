package migrations

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

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
