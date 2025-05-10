package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

// convertToFilePath converts the input string to a normalized file path and validates it.
func convertToFilePath(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	if !utf8.ValidString(input) {
		return "", fmt.Errorf("file path contains invalid UTF-8 characters")
	}

	if strings.ContainsRune(input, '\x00') {
		return "", fmt.Errorf("file path contains invalid null character")
	}

	normalizedFilePath := filepath.Clean(strings.Trim(input, "'"))

	err := validateNormalizedFilePath(normalizedFilePath)

	if err != nil {
		return "", fmt.Errorf("file path is not valid: %s", normalizedFilePath)
	}

	return normalizedFilePath, nil
}

// validateNormalizedFilePath validate whether a normalized file path is valid
func validateNormalizedFilePath(normalizedFilePath string) error {
	// Validate the normalized path format
	if runtime.GOOS == "windows" {
		// Windows-specific checks
		invalidChars := `<>"|?*`
		for _, char := range invalidChars {
			if strings.ContainsRune(normalizedFilePath, char) {
				return fmt.Errorf("file path contains invalid character: %c", char)
			}
		}

		// Check for reserved Windows names (e.g., CON, PRN, AUX)
		reservedNames := regexp.MustCompile(`^(?i)(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\..*)?$`)
		base := filepath.Base(normalizedFilePath)
		if reservedNames.MatchString(base) {
			return fmt.Errorf("file path uses reserved Windows name: %s", base)
		}

		// Check for trailing spaces or dots (invalid in Windows)
		if strings.HasSuffix(base, " ") || strings.HasSuffix(base, ".") {
			return fmt.Errorf("file path cannot end with a space or dot on Windows")
		}
	} else {
		// Unix-like systems: check for excessive length
		if len(normalizedFilePath) > 4096 { // Typical PATH_MAX limit
			return fmt.Errorf("file path is too long")
		}
	}

	return nil
}
