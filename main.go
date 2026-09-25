package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
)

func main() {
	// Parse command line flags
	helpFlag := flag.Bool("help", false, "Display help information")
	flag.Parse()

	// Show help if requested
	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	configs, err := loadConfig("config.yaml")

	if err != nil {
		pterm.Error.Printfln("Failed to read config file: %v", err)
		os.Exit(1)
	}

	// Create an interactive text input with single line input mode and show it
	projectPath := promptProjectPath()

	pterm.Printfln("Normalized file path is %s", pterm.Green(projectPath))

	selectedIDs := promptSelectConfigs(getOptionLabels(configs))

	selectedResources := getConfigMetadata(configs, selectedIDs)

	pterm.Printfln("Following files will be copied to the project path %s:", pterm.Green(projectPath))
	for _, fileToCopy := range selectedResources {
		dstPath, err := resolveDestinationPath(projectPath, fileToCopy)
		if err != nil {
			pterm.Error.Printfln("Invalid configuration for %s (%s): %v", fileToCopy.ID, fileToCopy.Path, err)
			os.Exit(1)
		}

		srcPath := filepath.Join(BASE_SOURCE_DIR, fileToCopy.Path)
		pterm.Printfln("- %s → %s.", pterm.Green(srcPath), pterm.Green(dstPath))
	}

	isConfirmed := promptConfirm("Do you want to proceed?")

	pterm.Println()

	if !isConfirmed {
		pterm.Error.Printfln("You cancelled copying!")
		os.Exit(0)
	}

	// Separate gitflow_init from regular file copying
	var regularResources []configResource
	var hasGitFlowInit bool
	for _, resource := range selectedResources {
		if resource.ID == "gitflow_init" {
			hasGitFlowInit = true
		} else {
			regularResources = append(regularResources, resource)
		}
	}

	// Copy regular files first
	if len(regularResources) > 0 {
		cm := NewCopyManager()
		err = cm.CopyFilesConcurrently(regularResources, projectPath)
		if err != nil {
			pterm.Error.Printfln("Error: %v", err)
			os.Exit(1)
		}
	}

	// Handle GitFlow initialization if selected
	if hasGitFlowInit {
		err = GitFlowInit(projectPath)
		if err != nil {
			pterm.Error.Printfln("Error initializing GitFlow: %v", err)
			os.Exit(1)
		}
	}
}

// printHelp displays usage information for the CLI
func printHelp() {
	fmt.Println("Usage: project-configs [options]")
	fmt.Println()
	fmt.Println("A tool for copying project configuration files to your projects.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help		Show this help message")
	fmt.Println()
	fmt.Println("Interactive mode:")
	fmt.Println("  The tool will guide you through selecting configuration files")
	fmt.Println("  and copying them to your project directory.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  project-configs		# Start interactive mode")
	fmt.Println("  project-configs --help	# Show this help message")
}
