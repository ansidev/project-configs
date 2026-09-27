package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
)

func main() {
	// Parse command line flags
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		pterm.Error.Printfln("Invalid arguments: %v", err)
		printHelp()
		os.Exit(1)
	}

	// Show help if requested
	if opts.help {
		printHelp()
		os.Exit(0)
	}

	configs, err := loadConfig(opts.configFile)

	if err != nil {
		pterm.Error.Printfln("Failed to read config file: %v", err)
		os.Exit(1)
	}

	// Check every referenced source file before the first prompt. Without this
	// the user only learns about a missing file after the whole interactive flow
	// has finished, and by then the other files have already been written.
	if err := validateSourceFiles(configs, opts.configDir); err != nil {
		pterm.Error.Printfln("%v", err)
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

		srcPath := filepath.Join(opts.configDir, fileToCopy.Path)
		pterm.Printfln("- %s → %s.", pterm.Green(srcPath), pterm.Green(dstPath))
	}

	isConfirmed := promptConfirm("Do you want to proceed?")

	pterm.Println()

	if !isConfirmed {
		pterm.Error.Printfln("You cancelled copying!")
		os.Exit(0)
	}

	// Separate gitflow_init from regular file copying
	gitFlowResource, regularResources := splitGitFlowResource(selectedResources)

	// Copy regular files first
	if len(regularResources) > 0 {
		cm := NewCopyManager(opts.configDir)
		err = cm.CopyFilesConcurrently(regularResources, projectPath)
		if err != nil {
			pterm.Error.Printfln("Error: %v", err)
			os.Exit(1)
		}
	}

	// Handle GitFlow initialization last, so that the generated files end up in
	// the initial commit
	if gitFlowResource != nil {
		err = GitFlowInit(projectPath, gitFlowResource.CommitMessage)
		if err != nil {
			pterm.Error.Printfln("Error initializing GitFlow: %v", err)
			os.Exit(1)
		}
	}
}

// splitGitFlowResource separates the GitFlow action resource from the file
// resources, preserving the order of the file resources. The GitFlow step must
// run after every file has been copied, so it is returned instead of being
// kept in the list. At most one GitFlow resource is returned, even when the
// selection contains more than one.
func splitGitFlowResource(resources []configResource) (gitFlowResource *configResource, fileResources []configResource) {
	for _, resource := range resources {
		if resource.ID == gitFlowResourceID {
			if gitFlowResource == nil {
				actionResource := resource
				gitFlowResource = &actionResource
			}
			continue
		}

		fileResources = append(fileResources, resource)
	}

	return gitFlowResource, fileResources
}

// printHelp displays usage information for the CLI
func printHelp() {
	fmt.Println("Usage: project-configs [options]")
	fmt.Println()
	fmt.Println("A tool for copying project configuration files to your projects.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help		Show this help message")
	fmt.Println("  -c, --config-file	Path to the config YAML file (default \"" + defaultConfigFile + "\")")
	fmt.Println("  -d, --config-dir	Path to the directory that contains the source configuration files (default \"" + defaultConfigDir + "\")")
	fmt.Println()
	fmt.Println("Interactive mode:")
	fmt.Println("  The tool will guide you through selecting configuration files")
	fmt.Println("  and copying them to your project directory.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  project-configs			# Start interactive mode")
	fmt.Println("  project-configs --help		# Show this help message")
	fmt.Println("  project-configs -c ./my-config.yaml	# Read configuration groups from another file")
	fmt.Println("  project-configs -d ./my-configs	# Copy source files from another directory")
}
