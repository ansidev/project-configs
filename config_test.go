package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveDestinationPath(t *testing.T) {
	tests := []struct {
		name      string
		destDir   string
		meta      configMetadata
		want      string
		wantError bool
	}{
		{
			name:    "uses source path when target is empty",
			destDir: "/tmp/project",
			meta:    configMetadata{Path: ".editorconfig"},
			want:    filepath.Join("/tmp/project", ".editorconfig"),
		},
		{
			name:    "renames file when target is set",
			destDir: "/tmp/project",
			meta:    configMetadata{Path: "MIT_LICENSE", Target: "LICENSE"},
			want:    filepath.Join("/tmp/project", "LICENSE"),
		},
		{
			name:    "target with directory replaces source directory",
			destDir: "/tmp/project",
			meta:    configMetadata{Path: "FUNDING.yml", Target: ".github/FUNDING.yml"},
			want:    filepath.Join("/tmp/project", ".github", "FUNDING.yml"),
		},
		{
			name:    "path with directories is preserved",
			destDir: "/tmp/project",
			meta:    configMetadata{Path: ".chglog/config.yml"},
			want:    filepath.Join("/tmp/project", ".chglog", "config.yml"),
		},
		{
			name:      "absolute path is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "/etc/passwd"},
			wantError: true,
		},
		{
			name:      "absolute target is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "MIT_LICENSE", Target: "/etc/LICENSE"},
			wantError: true,
		},
		{
			name:      "path escaping destination is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "../LICENSE"},
			wantError: true,
		},
		{
			name:      "target escaping destination is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "MIT_LICENSE", Target: "../LICENSE"},
			wantError: true,
		},
		{
			name:      "target escaping destination after internal parent is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "a/b.txt", Target: "a/../../evil.txt"},
			wantError: true,
		},
		{
			name:      "blank target is rejected",
			destDir:   "/tmp/project",
			meta:      configMetadata{Path: "MIT_LICENSE", Target: "   "},
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

	mit, ok := configs["MIT LICENSE"]
	if !ok {
		t.Fatal("loadConfig() missing 'MIT LICENSE' section")
	}
	if len(mit) != 1 {
		t.Fatalf("loadConfig() 'MIT LICENSE' should contain exactly 1 entry, got %d", len(mit))
	}
	if mit[0].Path != "MIT_LICENSE" {
		t.Errorf("MIT LICENSE path = %q, want %q", mit[0].Path, "MIT_LICENSE")
	}
	if mit[0].Target != "LICENSE" {
		t.Errorf("MIT LICENSE target = %q, want %q", mit[0].Target, "LICENSE")
	}
	if mit[0].PostMessage == "" {
		t.Error("MIT LICENSE post_message should not be empty")
	}
}

func TestGetOptionLabelsAndMetadata(t *testing.T) {
	configs := map[string][]configMetadata{
		"MIT LICENSE":   {{Path: "MIT_LICENSE", Target: "LICENSE", PostMessage: "update the variables"}},
		".editorconfig": {{Path: ".editorconfig"}},
	}

	labels := getOptionLabels(configs)
	if len(labels) != 2 {
		t.Errorf("getOptionLabels() = %v, want 2 labels", labels)
	}

	metadata := getConfigMetadata(configs, []string{"MIT LICENSE"})
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
