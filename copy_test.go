package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCopyFilesConcurrently(t *testing.T) {
	tests := []struct {
		name         string
		meta         configResource
		wantFileName string
		wantSubDir   string
	}{
		{
			name:         "target renames the destination file",
			meta:         configResource{Path: "MIT_LICENSE", Target: "LICENSE", PostMessage: "update the variables"},
			wantFileName: "LICENSE",
		},
		{
			name:         "no target keeps the source file name",
			meta:         configResource{Path: ".editorconfig"},
			wantFileName: ".editorconfig",
		},
		{
			name:         "target with directory creates the subdirectory",
			meta:         configResource{Path: "renovate.json", Target: ".github/renovate.json"},
			wantFileName: "renovate.json",
			wantSubDir:   ".github",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := t.TempDir()

			cm := NewCopyManager()
			if err := cm.CopyFilesConcurrently([]configResource{tt.meta}, destDir); err != nil {
				t.Fatalf("CopyFilesConcurrently() unexpected error: %v", err)
			}

			destPath := filepath.Join(destDir, tt.wantSubDir, tt.wantFileName)
			copied, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("CopyFilesConcurrently() did not create %s: %v", destPath, err)
			}

			source, err := os.ReadFile(filepath.Join(BASE_SOURCE_DIR, tt.meta.Path))
			if err != nil {
				t.Fatalf("failed to read source file: %v", err)
			}
			// Placeholders in templates (e.g. {current_year} in MIT_LICENSE)
			// are resolved during the copy, so the expected content is the
			// source content with all placeholders resolved.
			wantContent := strings.ReplaceAll(string(source), "{current_year}", strconv.Itoa(time.Now().Year()))
			if string(copied) != wantContent {
				t.Errorf("copied content differs from expected content")
			}

			if tt.meta.Target != "" {
				unwantedPath := filepath.Join(destDir, tt.meta.Path)
				if _, err := os.Stat(unwantedPath); err == nil {
					t.Errorf("CopyFilesConcurrently() should not create %s when target is %q", unwantedPath, tt.meta.Target)
				}
			}
		})
	}
}

func TestCopyFilesConcurrentlyRejectsInvalidPaths(t *testing.T) {
	destDir := t.TempDir()

	cm := NewCopyManager()
	err := cm.CopyFilesConcurrently([]configResource{{Path: "MIT_LICENSE", Target: "../escaped"}}, destDir)
	if err == nil {
		t.Fatal("CopyFilesConcurrently() expected error for path-traversal target")
	}

	if _, err := os.Stat(filepath.Join(destDir, "..", "escaped")); err == nil {
		t.Error("CopyFilesConcurrently() should not copy outside the destination directory")
	}
}

func TestResolveTemplatePlaceholders(t *testing.T) {
	values := templatePlaceholders(time.Date(2026, time.May, 17, 10, 0, 0, 0, time.UTC))
	if got := values["current_year"]; got != "2026" {
		t.Fatalf("templatePlaceholders() current_year = %q, want %q", got, "2026")
	}

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "replaces every occurrence of the placeholder",
			content: "Copyright (c) {current_year}-present\nYear: {current_year}\n",
			want:    "Copyright (c) 2026-present\nYear: 2026\n",
		},
		{
			name:    "content without the placeholder is unchanged",
			content: "no placeholders here\n",
			want:    "no placeholders here\n",
		},
		{
			name:    "unresolved placeholders are left untouched",
			content: "Copyright (c) {current_year} - {author_name}\n",
			want:    "Copyright (c) 2026 - {author_name}\n",
		},
		{
			name:    "similar tokens are not partially replaced",
			content: "{current_years} {current_year}{current_year}\n",
			want:    "{current_years} 20262026\n",
		},
		{
			name:    "empty content stays empty",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(resolveTemplatePlaceholders([]byte(tt.content), values))
			if got != tt.want {
				t.Errorf("resolveTemplatePlaceholders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopyFilesConcurrentlyResolvesCurrentYear(t *testing.T) {
	destDir := t.TempDir()

	cm := NewCopyManager()
	err := cm.CopyFilesConcurrently([]configResource{{Path: "MIT_LICENSE", Target: "LICENSE"}}, destDir)
	if err != nil {
		t.Fatalf("CopyFilesConcurrently() unexpected error: %v", err)
	}

	copied, err := os.ReadFile(filepath.Join(destDir, "LICENSE"))
	if err != nil {
		t.Fatalf("CopyFilesConcurrently() did not create LICENSE: %v", err)
	}

	content := string(copied)
	if strings.Contains(content, "{current_year}") {
		t.Error("copied LICENSE still contains the {current_year} placeholder")
	}

	wantCopyright := fmt.Sprintf("Copyright (c) %d-present", time.Now().Year())
	if !strings.Contains(content, wantCopyright) {
		t.Errorf("copied LICENSE should contain %q, got:\n%s", wantCopyright, content)
	}

	if !strings.Contains(content, "{author_name}") {
		t.Error("copied LICENSE should keep the {author_name} placeholder untouched")
	}
}
