package main

import (
	"os"
	"path/filepath"
	"testing"
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
			if string(copied) != string(source) {
				t.Errorf("copied content differs from source content")
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
