package utils

import (
	"strings"
)

func SanitizeFilename(path string) string {
	return strings.ReplaceAll(path, "/", "-")
}
