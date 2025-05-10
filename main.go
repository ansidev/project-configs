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
	inputPath, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultText("1. Project path").
		Show()

	projectPath, err := convertToFilePath(inputPath)

	if err != nil {
		pterm.Error.Printf("Failed to input file path: %v\n", err)
		os.Exit(1)
	}

	pterm.Printfln("Normalized file path is %s", pterm.Green(projectPath))

	selectedConfigs, _ := pterm.DefaultInteractiveMultiselect.
		WithDefaultText("2. Which configurations do you want to copy to your project?").
		WithOptions(getOptionLabels(configs)).
		WithFilter(true).
		Show()

	configMetadata := getConfigMetadata(configs, selectedConfigs)

	pterm.Printfln("Following file will be copied to the project path %s:", pterm.Green(projectPath))
	for _, fileToCopy := range configMetadata {
		pterm.Printfln("- %s.", pterm.Green(filepath.Join(BASE_SOURCE_DIR, fileToCopy.Path)))
	}

	isConfirmed, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultText("Do you want to proceed?").
		Show()

	pterm.Println()

	if !isConfirmed {
		pterm.Error.Printfln("You cancelled copying!")
		os.Exit(0)
	}
	cm := NewCopyManager()
	err = cm.CopyFilesConcurrently(configMetadata, projectPath)
	if err != nil {
		pterm.Error.Printfln("Error: %v", err)
		os.Exit(1)
	}
}
