package main

import (
	"strings"
	"testing"
)

// TestGitFlowStepRunsAfterFileCopy walks the same sequence as main: flatten the
// selected groups, split the GitFlow action resource out, copy the remaining
// files and only then initialize GitFlow. It covers R1.1/R1.2 (GitFlow last)
// and R3.1/R3.3 (the copied files are committed to the production release
// branch with the message configured in config.yaml).
func TestGitFlowStepRunsAfterFileCopy(t *testing.T) {
	configs, err := loadConfig("config.yaml")
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	selectedIDs := []string{"editorconfig", "mit_license", gitFlowResourceID}
	selectedResources := getConfigMetadata(configs, selectedIDs)

	gitFlowResource, fileResources := splitGitFlowResource(selectedResources)
	if gitFlowResource == nil {
		t.Fatal("splitGitFlowResource() did not find the git flow resource")
	}
	if len(fileResources) != 2 {
		t.Fatalf("splitGitFlowResource() returned %d file resources, want 2", len(fileResources))
	}

	projectPath := t.TempDir()

	cm := NewCopyManager(defaultConfigDir)
	if err := cm.CopyFilesConcurrently(fileResources, projectPath); err != nil {
		t.Fatalf("CopyFilesConcurrently() unexpected error: %v", err)
	}

	if err := GitFlowInit(projectPath, gitFlowResource.CommitMessage); err != nil {
		t.Fatalf("GitFlowInit() unexpected error: %v", err)
	}

	productionBranch := productionBranchOf(t, projectPath)

	// Every copied file must be part of the commit on the production release
	// branch, which is only possible when GitFlow ran after the copy.
	tracked := gitOutput(t, projectPath, "ls-tree", "-r", "--name-only", productionBranch)
	for _, want := range []string{".editorconfig", "LICENSE"} {
		if !strings.Contains(tracked, want) {
			t.Errorf("commit on %s is missing %s, tracked files: %s", productionBranch, want, tracked)
		}
	}

	subjects := commitSubjects(t, projectPath, productionBranch)
	if len(subjects) != 1 || subjects[0] != gitFlowResource.CommitMessage {
		t.Errorf("commits on %s = %v, want exactly [%q]", productionBranch, subjects, gitFlowResource.CommitMessage)
	}
}

func TestSplitGitFlowResource(t *testing.T) {
	gitFlow := configResource{ID: gitFlowResourceID, CommitMessage: "chore: initial commit"}

	tests := []struct {
		name          string
		resources     []configResource
		wantGitFlow   *configResource
		wantFilePaths []string
	}{
		{
			name:      "no resources",
			resources: nil,
		},
		{
			name: "git flow is extracted and removed from the file resources",
			resources: []configResource{
				{ID: "editorconfig", Path: ".editorconfig"},
				gitFlow,
				{ID: "mit_license", Path: "MIT_LICENSE", Target: "LICENSE"},
			},
			wantGitFlow:   &gitFlow,
			wantFilePaths: []string{".editorconfig", "MIT_LICENSE"},
		},
		{
			name: "git flow first in the selection is still extracted",
			resources: []configResource{
				gitFlow,
				{ID: "editorconfig", Path: ".editorconfig"},
			},
			wantGitFlow:   &gitFlow,
			wantFilePaths: []string{".editorconfig"},
		},
		{
			name: "file resources keep their selection order",
			resources: []configResource{
				{ID: "renovate_json", Path: "renovate.json"},
				gitFlow,
				{ID: "editorconfig", Path: ".editorconfig"},
			},
			wantGitFlow:   &gitFlow,
			wantFilePaths: []string{"renovate.json", ".editorconfig"},
		},
		{
			name: "only the first git flow resource is returned",
			resources: []configResource{
				{ID: gitFlowResourceID, CommitMessage: "first"},
				{ID: "editorconfig", Path: ".editorconfig"},
				{ID: gitFlowResourceID, CommitMessage: "second"},
			},
			wantGitFlow:   &configResource{ID: gitFlowResourceID, CommitMessage: "first"},
			wantFilePaths: []string{".editorconfig"},
		},
		{
			name: "resources without the git flow id are all file resources",
			resources: []configResource{
				{ID: "editorconfig", Path: ".editorconfig"},
			},
			wantFilePaths: []string{".editorconfig"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGitFlow, gotFiles := splitGitFlowResource(tt.resources)

			switch {
			case tt.wantGitFlow == nil && gotGitFlow != nil:
				t.Fatalf("splitGitFlowResource() git flow resource = %+v, want nil", gotGitFlow)
			case tt.wantGitFlow != nil && gotGitFlow == nil:
				t.Fatalf("splitGitFlowResource() git flow resource = nil, want %+v", tt.wantGitFlow)
			case tt.wantGitFlow != nil && *gotGitFlow != *tt.wantGitFlow:
				t.Errorf("splitGitFlowResource() git flow resource = %+v, want %+v", *gotGitFlow, *tt.wantGitFlow)
			}

			if len(gotFiles) != len(tt.wantFilePaths) {
				t.Fatalf("splitGitFlowResource() returned %d file resources, want %d", len(gotFiles), len(tt.wantFilePaths))
			}
			for i, want := range tt.wantFilePaths {
				if gotFiles[i].Path != want {
					t.Errorf("file resource %d = %q, want %q", i, gotFiles[i].Path, want)
				}
			}
		})
	}
}
