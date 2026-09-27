package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		{
			name:      "empty path returns destDir for action resources",
			destDir:   "/tmp/project",
			meta:      configResource{Path: ""},
			want:      "/tmp/project",
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
		"agpl_v3_license":     "AGPL v3 LICENSE",
		"github_funding":      "GitHub Funding",
		"gitflow_init":        "Initialize GitFlow",
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

// newTestConfigs returns configs with a single group holding the given
// resources, for use in validateSourceFiles tests.
func newTestConfigs(resources ...configResource) map[string]configGroup {
	return map[string]configGroup{
		"editorconfig": {
			Label:     ".editorconfig",
			Resources: resources,
		},
	}
}

// TestValidateSourceFilesAccepts verifies that a config whose source files all
// exist under the config directory passes validation, including nested paths
// that resolve through intermediate directories.
func TestValidateSourceFilesAccepts(t *testing.T) {
	sourceDir := t.TempDir()
	for _, path := range []string{"renovate.json", ".editorconfig", ".chglog/config.yml"} {
		full := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte("content"), 0o644); err != nil {
			t.Fatalf("failed to create source file %s: %v", path, err)
		}
	}

	configs := newTestConfigs(
		configResource{ID: "renovate", Path: "renovate.json"},
		configResource{ID: "editorconfig", Path: ".editorconfig"},
		configResource{ID: "chglog", Path: ".chglog/config.yml"},
	)

	if err := validateSourceFiles(configs, sourceDir); err != nil {
		t.Errorf("validateSourceFiles() unexpected error: %v", err)
	}
}

// TestValidateSourceFilesRejects verifies that an unusable source path is
// reported with enough detail to identify the config.yaml line to fix: the
// group id, the resource id and the resolved path.
func TestValidateSourceFilesRejects(t *testing.T) {
	sourceDir := t.TempDir()
	// A directory in place of a file is a different defect from a missing file
	// and must be described differently.
	if err := os.MkdirAll(filepath.Join(sourceDir, "chglog"), 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	tests := []struct {
		name     string
		resource configResource
		wantErr  []string
	}{
		{
			name:     "missing file is reported",
			resource: configResource{ID: "renovate", Path: "renovate.json"},
			wantErr:  []string{"group \"editorconfig\"", "resource \"renovate\"", "renovate.json", "does not exist"},
		},
		{
			name:     "missing file in a nested directory is reported",
			resource: configResource{ID: "chglog", Path: ".chglog/config.yml"},
			wantErr:  []string{"group \"editorconfig\"", "resource \"chglog\"", filepath.Join(".chglog", "config.yml"), "does not exist"},
		},
		{
			name:     "directory in place of a file is reported as not a regular file",
			resource: configResource{ID: "chglog", Path: "chglog"},
			wantErr:  []string{"resource \"chglog\"", "is not a regular file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceFiles(newTestConfigs(tt.resource), sourceDir)
			if err == nil {
				t.Fatalf("validateSourceFiles() expected error for path %q, got nil", tt.resource.Path)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("validateSourceFiles() error = %q, want it to contain %q", err.Error(), want)
				}
			}
			if !strings.Contains(err.Error(), sourceDir) {
				t.Errorf("validateSourceFiles() error = %q, want it to contain the resolved source dir %q", err.Error(), sourceDir)
			}
		})
	}
}

// TestValidateSourceFilesSkipsActionResources verifies that resources with an
// empty path, such as gitflow_init, are not treated as missing files: they
// copy nothing and reference no source.
func TestValidateSourceFilesSkipsActionResources(t *testing.T) {
	configs := map[string]configGroup{
		"gitflow_init": {
			Label: "Initialize GitFlow",
			Resources: []configResource{
				{ID: "gitflow_init", Path: "", PostMessage: "GitFlow has been initialized for your project."},
			},
		},
	}

	if err := validateSourceFiles(configs, t.TempDir()); err != nil {
		t.Errorf("validateSourceFiles() unexpected error for an action resource: %v", err)
	}
}

// TestValidateSourceFilesReportsEveryProblem verifies that all unusable paths
// are reported in one pass, so the user does not have to fix them one run at a
// time.
func TestValidateSourceFilesReportsEveryProblem(t *testing.T) {
	sourceDir := t.TempDir()
	configs := map[string]configGroup{
		"changelog_generator": {
			Label: "changelog generator",
			Resources: []configResource{
				{ID: "chglog_config", Path: ".chglog/config.yml"},
				{ID: "changelog_template", Path: ".chglog/CHANGELOG.tpl.md"},
			},
		},
		"editorconfig": {
			Label:     ".editorconfig",
			Resources: []configResource{{ID: "editorconfig", Path: ".editorconfig"}},
		},
	}

	err := validateSourceFiles(configs, sourceDir)
	if err == nil {
		t.Fatal("validateSourceFiles() expected error for missing source files, got nil")
	}
	for _, want := range []string{"chglog_config", "changelog_template", "editorconfig"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("validateSourceFiles() error = %q, want it to report resource %q", err.Error(), want)
		}
	}
}

// TestValidateSourceFilesOrderIsDeterministic verifies that problems are
// reported in a stable order. Go randomises map iteration, so without sorting
// the group ids the same broken config would produce a different message on
// every run.
func TestValidateSourceFilesOrderIsDeterministic(t *testing.T) {
	sourceDir := t.TempDir()
	configs := map[string]configGroup{
		"changelog_generator": {Label: "changelog generator", Resources: []configResource{{ID: "chglog_config", Path: ".chglog/config.yml"}}},
		"editorconfig":        {Label: ".editorconfig", Resources: []configResource{{ID: "editorconfig", Path: ".editorconfig"}}},
		"mit_license":         {Label: "MIT LICENSE", Resources: []configResource{{ID: "mit_license", Path: "MIT_LICENSE"}}},
		"renovate_json":       {Label: "renovate.json", Resources: []configResource{{ID: "renovate_json", Path: "renovate.json"}}},
	}

	first := validateSourceFiles(configs, sourceDir)
	if first == nil {
		t.Fatal("validateSourceFiles() expected error for missing source files, got nil")
	}
	for i := 0; i < 20; i++ {
		if got := validateSourceFiles(configs, sourceDir); got.Error() != first.Error() {
			t.Fatalf("validateSourceFiles() error is not deterministic:\nfirst: %q\n  got: %q", first.Error(), got.Error())
		}
	}

	// Group ids are reported sorted, resources in file order.
	wantOrder := []string{"changelog_generator", "editorconfig", "mit_license", "renovate_json"}
	previous := -1
	for _, groupID := range wantOrder {
		index := strings.Index(first.Error(), groupID)
		if index == -1 {
			t.Fatalf("validateSourceFiles() error = %q, want it to mention group %q", first.Error(), groupID)
		}
		if index < previous {
			t.Errorf("validateSourceFiles() error = %q, want group %q reported in sorted order", first.Error(), groupID)
		}
		previous = index
	}
}

// TestValidateSourceFilesAcceptsShippedConfig is a regression guard for the
// strict check: the config.yaml that ships with the tool must resolve against
// the configs directory that ships with it, otherwise every default
// invocation would fail before the first prompt.
func TestValidateSourceFilesAcceptsShippedConfig(t *testing.T) {
	configs, err := loadConfig(defaultConfigFile)
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	if err := validateSourceFiles(configs, defaultConfigDir); err != nil {
		t.Errorf("validateSourceFiles() unexpected error for the shipped config: %v", err)
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
