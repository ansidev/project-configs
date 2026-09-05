package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveDestinationPath(t *testing.T) {
	tests := []struct {
		name      string
		destDir   string
		meta      configResource
		want      string
		wantError bool
	}{
		{
			name:    "uses source path when target is empty",
			destDir: "/tmp/project",
			meta:    configResource{Path: ".editorconfig"},
			want:    filepath.Join("/tmp/project", ".editorconfig"),
		},
		{
			name:    "renames file when target is set",
			destDir: "/tmp/project",
			meta:    configResource{Path: "MIT_LICENSE", Target: "LICENSE"},
			want:    filepath.Join("/tmp/project", "LICENSE"),
		},
		{
			name:    "target with directory replaces source directory",
			destDir: "/tmp/project",
			meta:    configResource{Path: "FUNDING.yml", Target: ".github/FUNDING.yml"},
			want:    filepath.Join("/tmp/project", ".github", "FUNDING.yml"),
		},
		{
			name:    "path with directories is preserved",
			destDir: "/tmp/project",
			meta:    configResource{Path: ".chglog/config.yml"},
			want:    filepath.Join("/tmp/project", ".chglog", "config.yml"),
		},
		{
			name:      "absolute path is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "/etc/passwd"},
			wantError: true,
		},
		{
			name:      "absolute target is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "MIT_LICENSE", Target: "/etc/LICENSE"},
			wantError: true,
		},
		{
			name:      "path escaping destination is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "../LICENSE"},
			wantError: true,
		},
		{
			name:      "target escaping destination is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "MIT_LICENSE", Target: "../LICENSE"},
			wantError: true,
		},
		{
			name:      "target escaping destination after internal parent is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "a/b.txt", Target: "a/../../evil.txt"},
			wantError: true,
		},
		{
			name:      "blank target is rejected",
			destDir:   "/tmp/project",
			meta:      configResource{Path: "MIT_LICENSE", Target: "   "},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDestinationPath(tt.destDir, tt.meta)

			if tt.wantError {
				if err == nil {
					t.Fatalf("resolveDestinationPath(%q, %+v) expected error, got %q", tt.destDir, tt.meta, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveDestinationPath(%q, %+v) unexpected error: %v", tt.destDir, tt.meta, err)
			}

			if got != tt.want {
				t.Errorf("resolveDestinationPath(%q, %+v) = %q, want %q", tt.destDir, tt.meta, got, tt.want)
			}
		})
	}
}

func TestValidateRelativePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{name: "simple filename is valid", path: "LICENSE"},
		{name: "nested path is valid", path: ".github/workflows/publish_release.yaml"},
		{name: "dot directory is valid", path: "./LICENSE"},
		{name: "internal parent directory is valid", path: ".chglog/../.chglog/config.yml"},
		{name: "empty path is invalid", path: "", wantError: true},
		{name: "blank path is invalid", path: "   ", wantError: true},
		{name: "dot-dot filename is invalid", path: "..", wantError: true},
		{name: "dot segment escape is invalid", path: "./../LICENSE", wantError: true},
		{name: "parent escape is invalid", path: "../LICENSE", wantError: true},
		{name: "deeply nested escape is invalid", path: "a/b/../../../LICENSE", wantError: true},
		{name: "absolute path is invalid", path: "/etc/LICENSE", wantError: true},
	}

	// Windows rejects additional characters and reserved names that are valid on Unix.
	if runtime.GOOS == "windows" {
		windowsInvalidPaths := []struct {
			name string
			path string
		}{
			{name: "absolute windows path is invalid", path: `C:\temp\LICENSE`},
			{name: "angle bracket character is invalid", path: `file<>name`},
			{name: "pipe character is invalid", path: `file|name`},
			{name: "colon character is invalid", path: `file:name`},
			{name: "question mark character is invalid", path: `file?name`},
			{name: "asterisk character is invalid", path: `file*name`},
			{name: "quote character is invalid", path: `file"name`},
			{name: "reserved windows name con is invalid", path: "con.txt"},
			{name: "reserved windows name aux is invalid", path: "aux.yaml"},
			{name: "trailing dot is invalid", path: "LICENSE."},
			{name: "trailing space is invalid", path: "LICENSE "},
		}

		for _, invalidPath := range windowsInvalidPaths {
			tests = append(tests, struct {
				name      string
				path      string
				wantError bool
			}{name: invalidPath.name, path: invalidPath.path, wantError: true})
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRelativePath(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("validateRelativePath(%q) error = %v, wantError %v", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	configs, err := loadConfig("config.yaml")
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	mit, ok := configs["mit_license"]
	if !ok {
		t.Fatal("loadConfig() missing 'mit_license' group")
	}
	if mit.Label != "MIT LICENSE" {
		t.Errorf("mit_license label = %q, want %q", mit.Label, "MIT LICENSE")
	}
	if len(mit.Resources) != 1 {
		t.Fatalf("loadConfig() 'mit_license' should contain exactly 1 resource, got %d", len(mit.Resources))
	}
	resource := mit.Resources[0]
	if resource.ID != "mit_license" {
		t.Errorf("mit_license resource id = %q, want %q", resource.ID, "mit_license")
	}
	if resource.Path != "MIT_LICENSE" {
		t.Errorf("mit_license path = %q, want %q", resource.Path, "MIT_LICENSE")
	}
	if resource.Target != "LICENSE" {
		t.Errorf("mit_license target = %q, want %q", resource.Target, "LICENSE")
	}
	if resource.PostMessage == "" {
		t.Error("mit_license post_message should not be empty")
	}

	// Every config group must be present with the exact configured label.
	wantLabels := map[string]string{
		"git_flow_for_github": "git-flow for GitHub",
		"changelog_generator": "changelog generator",
		"editorconfig":        ".editorconfig",
		"renovate_json":       "renovate.json",
		"mit_license":         "MIT LICENSE",
		"github_funding":      "GitHub Funding",
	}
	for groupID, label := range wantLabels {
		group, ok := configs[groupID]
		if !ok {
			t.Errorf("loadConfig() missing group %q", groupID)
			continue
		}
		if group.Label != label {
			t.Errorf("group %q label = %q, want %q", groupID, group.Label, label)
		}
	}
	// The config must not contain any unexpected groups.
	for groupID, group := range configs {
		if wantLabels[groupID] == "" {
			t.Errorf("loadConfig() unexpected group %q with label %q", groupID, group.Label)
		}
	}
}

func TestGetOptionLabelsAndMetadata(t *testing.T) {
	configs := map[string]configGroup{
		"mit_license": {
			Label: "MIT LICENSE",
			Resources: []configResource{
				{ID: "mit_license", Path: "MIT_LICENSE", Target: "LICENSE", PostMessage: "update the variables"},
			},
		},
		"editorconfig": {
			Label:     ".editorconfig",
			Resources: []configResource{{ID: "editorconfig", Path: ".editorconfig"}},
		},
	}

	options := getOptionLabels(configs)
	if len(options) != 2 {
		t.Errorf("getOptionLabels() = %v, want 2 options", options)
	}
	// Options must be sorted by group id and carry the group label.
	if len(options) > 1 && options[0].ID > options[1].ID {
		t.Errorf("getOptionLabels() not sorted by id: %v", options)
	}
	for _, option := range options {
		if configs[option.ID].Label != option.Label {
			t.Errorf("option %q has label %q, want %q", option.ID, option.Label, configs[option.ID].Label)
		}
	}

	metadata := getConfigMetadata(configs, []string{"mit_license"})
	if len(metadata) != 1 {
		t.Fatalf("getConfigMetadata() returned %d entries, want 1", len(metadata))
	}
	if metadata[0].Target != "LICENSE" {
		t.Errorf("getConfigMetadata() target = %q, want %q", metadata[0].Target, "LICENSE")
	}

	if metadata := getConfigMetadata(configs, []string{"unknown"}); len(metadata) != 0 {
		t.Errorf("getConfigMetadata(unknown) = %v, want empty", metadata)
	}
}

// writeTempConfig writes the given YAML content to a temp file and returns its
// path, for use in loadConfig tests.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// TestLoadConfigValidation covers schema validation: invalid group keys,
// missing labels/resources, and invalid or duplicate resource ids must be
// rejected with an error.
func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "group key not snake_case is rejected",
			content: "\"git-flow for GitHub\":\n  label: git-flow for GitHub\n  resources:\n    - id: rebase\n      path: rebase.yaml\n",
		},
		{
			name:    "missing label is rejected",
			content: "git_flow:\n  resources:\n    - id: rebase\n      path: rebase.yaml\n",
		},
		{
			name:    "blank label is rejected",
			content: "git_flow:\n  label: \"  \"\n  resources:\n    - id: rebase\n      path: rebase.yaml\n",
		},
		{
			name:    "missing resources is rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n",
		},
		{
			name:    "empty resources is rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n  resources: []\n",
		},
		{
			name:    "missing resource id is rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n  resources:\n    - path: rebase.yaml\n",
		},
		{
			name:    "empty resource id is rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n  resources:\n    - id: \"\"\n      path: rebase.yaml\n",
		},
		{
			name:    "non snake_case resource id is rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n  resources:\n    - id: rebaseWorkflow\n      path: rebase.yaml\n",
		},
		{
			name:    "duplicate resource ids are rejected",
			content: "git_flow:\n  label: git-flow for GitHub\n  resources:\n    - id: rebase\n      path: rebase.yaml\n    - id: rebase\n      path: other.yaml\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempConfig(t, tt.content)
			if _, err := loadConfig(path); err == nil {
				t.Fatalf("loadConfig() expected error for content:\n%s", tt.content)
			}
		})
	}
}

// TestLoadConfigValidConfigAccepts ensures a valid minimal config passes
// validation.
func TestLoadConfigValidConfigAccepts(t *testing.T) {
	path := writeTempConfig(t, "editorconfig:\n  label: .editorconfig\n  resources:\n    - id: editorconfig\n      path: .editorconfig\n")

	configs, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}
	group, ok := configs["editorconfig"]
	if !ok {
		t.Fatal("loadConfig() missing 'editorconfig' group")
	}
	if group.Label != ".editorconfig" {
		t.Errorf("label = %q, want %q", group.Label, ".editorconfig")
	}
	if len(group.Resources) != 1 || group.Resources[0].ID != "editorconfig" {
		t.Errorf("resources = %+v, want a single resource with id 'editorconfig'", group.Resources)
	}
}
