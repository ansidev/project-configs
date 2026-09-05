package main

import (
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
)

func main() {
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
	cm := NewCopyManager()
	err = cm.CopyFilesConcurrently(selectedResources, projectPath)
	if err != nil {
		pterm.Error.Printfln("Error: %v", err)
		os.Exit(1)
	}
}
