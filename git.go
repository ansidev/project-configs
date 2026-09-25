package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
)

// GitFlowInit initializes git and git flow for a project
func GitFlowInit(projectPath string) error {
	var results []string

	// Check if git repository already exists
	gitInitDone, err := isGitRepoInitialized(projectPath)
	if err != nil {
		return fmt.Errorf("failed to check git repository status: %v", err)
	}

	if !gitInitDone {
		// Initialize git repository
		err = runGitInit(projectPath)
		if err != nil {
			return fmt.Errorf("failed to initialize git repository: %v", err)
		}
		results = append(results, "✓ Git repository initialized")
	} else {
		results = append(results, "○ Git repository already initialized (skipped)")
	}

	// Check if git flow is already initialized
	gitFlowInitDone, err := isGitFlowInitialized(projectPath)
	if err != nil {
		return fmt.Errorf("failed to check git flow status: %v", err)
	}

	if !gitFlowInitDone {
		// Initialize git flow
		err = runGitFlowInit(projectPath)
		if err != nil {
			return fmt.Errorf("failed to initialize git flow: %v", err)
		}
		results = append(results, "✓ Git Flow initialized")
	} else {
		results = append(results, "○ Git Flow already initialized (skipped)")
	}

	// Print summary
	pterm.Println()
	pterm.Info.Println("GitFlow Initialization Summary:")
	for _, result := range results {
		pterm.Printfln("  %s", result)
	}

	return nil
}

// isGitRepoInitialized checks if a git repository is already initialized
func isGitRepoInitialized(projectPath string) (bool, error) {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// runGitInit executes git init in the project directory
func runGitInit(projectPath string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = projectPath
	
	pterm.Printfln("Running: %s in %s", pterm.Green("git init"), pterm.Green(projectPath))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init failed: %v, output: %s", err, string(output))
	}

	return nil
}

// isGitFlowInitialized checks if git flow is already initialized
func isGitFlowInitialized(projectPath string) (bool, error) {
	// Check if git flow branch structure exists
	// We'll check for the presence of develop branch which is created by git flow init
	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = projectPath
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If git command fails, it might mean git is not initialized at all
		// or there's some other issue - we'll return false to let git flow init handle it
		return false, nil
	}

	// Check if develop branch exists
	if strings.Contains(string(output), "develop") {
		return true, nil
	}

	return false, nil
}

// runGitFlowInit executes git flow init with default options
func runGitFlowInit(projectPath string) error {
	cmd := exec.Command("git", "flow", "init", "-d")
	cmd.Dir = projectPath
	
	pterm.Printfln("Running: %s in %s", pterm.Green("git flow init -d"), pterm.Green(projectPath))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git flow init failed: %v, output: %s", err, string(output))
	}

	return nil
}