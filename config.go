package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type configResource struct {
	ID          string `yaml:"id"`
	Path        string `yaml:"path"`
	Target      string `yaml:"target,omitempty"`
	PostMessage string `yaml:"post_message"`
}

type configGroup struct {
	Label     string           `yaml:"label"`
	Resources []configResource `yaml:"resources"`
}

// configOption pairs a stable group id with the human-readable label shown in
// prompts. Selection values carry the ID while the UI displays the Label.
type configOption struct {
	ID    string
	Label string
}

// snakeCasePattern matches non-empty snake_case identifiers:
// lowercase letters/digits with underscores, starting with a letter.
var snakeCasePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateSnakeCase returns an error if the given identifier is not a valid
// non-empty snake_case string.
func validateSnakeCase(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", kind)
	}
	if !snakeCasePattern.MatchString(value) {
		return fmt.Errorf("%s %q must be snake_case (lowercase letters, digits and underscores)", kind, value)
	}
	return nil
}

func loadConfig(filePath string) (map[string]configGroup, error) {
	// Read the YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Unmarshal YAML into a map[string]configGroup
	var result map[string]configGroup
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	if err := validateConfig(result); err != nil {
		return nil, err
	}

	return result, nil
}

// validateConfig enforces the config schema invariants: snake_case group keys,
// non-empty labels, non-empty resource lists, and unique snake_case resource
// ids within each group.
func validateConfig(configs map[string]configGroup) error {
	for groupID, group := range configs {
		if err := validateSnakeCase("group id", groupID); err != nil {
			return err
		}
		if strings.TrimSpace(group.Label) == "" {
			return fmt.Errorf("group %q must have a non-empty label", groupID)
		}
		if len(group.Resources) == 0 {
			return fmt.Errorf("group %q must have at least one resource", groupID)
		}

		seenIDs := make(map[string]struct{}, len(group.Resources))
		for _, resource := range group.Resources {
			if err := validateSnakeCase("resource id", resource.ID); err != nil {
				return fmt.Errorf("group %q: %v", groupID, err)
			}
			if _, exists := seenIDs[resource.ID]; exists {
				return fmt.Errorf("group %q has duplicate resource id %q", groupID, resource.ID)
			}
			seenIDs[resource.ID] = struct{}{}
		}
	}
	return nil
}

// getOptionLabels returns the selectable options in a deterministic order,
// sorted by group id.
func getOptionLabels(configs map[string]configGroup) []configOption {
	ids := make([]string, 0, len(configs))
	for id := range configs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	options := make([]configOption, 0, len(ids))
	for _, id := range ids {
		options = append(options, configOption{ID: id, Label: configs[id].Label})
	}
	return options
}

func getConfigMetadata(configs map[string]configGroup, selectedIDs []string) []configResource {
	var selectedResources []configResource

	for _, selectedID := range selectedIDs {
		group, exist := configs[selectedID]
		if exist {
			selectedResources = append(selectedResources, group.Resources...)
		}
	}
	return selectedResources
}

// resolveDestinationPath resolves the destination path for a config resource
// inside destDir. When the resource specifies a target, the target replaces the
// source path (including any directory components). Otherwise the source path
// is used as-is.
func resolveDestinationPath(destDir string, resource configResource) (string, error) {
	relPath := resource.Path
	if resource.Target != "" {
		relPath = resource.Target
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
