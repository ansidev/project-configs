package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type configMetadata struct {
	Path        string `yaml:"path"`
	Target      string `yaml:"target,omitempty"`
	PostMessage string `yaml:"post_message"`
}

func loadConfig(filePath string) (map[string][]configMetadata, error) {
	// Read the YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Unmarshal YAML into a map[string][]Option
	var result map[string][]configMetadata
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	return result, nil
}

func getOptionLabels(configs map[string][]configMetadata) []string {
	var labels []string
	for label := range configs {
		labels = append(labels, label)
	}
	return labels
}

func getConfigMetadata(configs map[string][]configMetadata, selectedConfigs []string) []configMetadata {
	var configMetadata []configMetadata

	for _, selectedConfig := range selectedConfigs {
		selectedConfigMetadata, exist := configs[selectedConfig]
		if exist {
			configMetadata = append(configMetadata, selectedConfigMetadata...)
		}
	}
	return configMetadata
}

// resolveDestinationPath resolves the destination path for a config entry
// inside destDir. When the entry specifies a target, the target replaces the
// source path (including any directory components). Otherwise the source path
// is used as-is.
func resolveDestinationPath(destDir string, meta configMetadata) (string, error) {
	relPath := meta.Path
	if meta.Target != "" {
		relPath = meta.Target
	}

	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}

	return filepath.Join(destDir, relPath), nil
}

// validateRelativePath ensures the given path is a safe relative path: it must
// not be empty, absolute, or escape the destination directory.
func validateRelativePath(relPath string) error {
	if strings.TrimSpace(relPath) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if filepath.IsAbs(relPath) {
		return fmt.Errorf("path %s must be relative", relPath)
	}

	// Reject path-traversal segments that would escape the destination directory.
	depth := 0
	for _, segment := range strings.Split(filepath.ToSlash(relPath), "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if depth == 0 {
				return fmt.Errorf("path %s must not escape the destination directory", relPath)
			}
			depth--
		default:
			depth++
		}
	}

	if runtime.GOOS == "windows" {
		// Windows-specific checks
		invalidChars := `<>:"|?*`
		for _, char := range invalidChars {
			if strings.ContainsRune(relPath, char) {
				return fmt.Errorf("path contains invalid character: %c", char)
			}
		}

		// Check for reserved Windows names (e.g., CON, PRN, AUX)
		reservedNames := regexp.MustCompile(`^(?i)(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\..*)?$`)
		base := filepath.Base(relPath)
		if reservedNames.MatchString(base) {
			return fmt.Errorf("path uses reserved Windows name: %s", base)
		}

		// Check for trailing spaces or dots (invalid in Windows)
		if strings.HasSuffix(base, " ") || strings.HasSuffix(base, ".") {
			return fmt.Errorf("path cannot end with a space or dot on Windows")
		}
	}

	return nil
}
