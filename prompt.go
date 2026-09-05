package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/pterm/pterm"
)

// runForm runs a single huh field wrapped in its own form so that the field
// position and help bar are rendered correctly.
//
// If the user aborts the prompt (Esc / Ctrl+C), the program exits gracefully
// with a cancellation notice and exit code 0.
func runForm(field huh.Field) {
	form := huh.NewForm(huh.NewGroup(field)).WithShowHelp(true)
	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			pterm.Warning.Println("Copying cancelled by user.")
			os.Exit(0)
		}
		pterm.Error.Printfln("Prompt failed: %v", err)
		os.Exit(1)
	}
}

// promptProjectPath asks for the target project path and validates it.
// Invalid paths re-prompt inline via the form validation mechanism.
func promptProjectPath() string {
	var projectPath string
	input := huh.NewInput().
		Title("1. Project path").
		Validate(func(s string) error {
			_, err := convertToFilePath(s)
			return err
		}).
		Value(&projectPath)

	runForm(input)

	normalized, err := convertToFilePath(projectPath)
	if err != nil {
		// Should not happen because the form already validated the input.
		pterm.Error.Printf("Failed to input file path: %v\n", err)
		os.Exit(1)
	}
	return normalized
}

// newConfigOptions wraps config options into huh options so that the
// multiselect displays each group's label while the multiselect value
// ([]string) contains the selected group ids.
func newConfigOptions(options []configOption) []huh.Option[string] {
	huhOptions := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		huhOptions = append(huhOptions, huh.NewOption(option.Label, option.ID))
	}
	return huhOptions
}

// promptSelectConfigs shows an interactive multiselect where Space toggles an
// option's selection and Enter confirms the current selection. Pressing "/"
// enters filter mode; Enter applies the filter and leaves filter mode without
// submitting; Esc exits filter mode and clears the filter.
func promptSelectConfigs(options []configOption) []string {
	var selected []string

	height := len(options)
	if height < 6 {
		height = 6
	}

	multiselect := huh.NewMultiSelect[string]().
		Title("2. Which configurations do you want to copy to your project?").
		Options(newConfigOptions(options)...).
		Height(height).
		Value(&selected)

	runForm(multiselect)
	return selected
}

// promptConfirm shows a yes/no confirmation prompt.
func promptConfirm(message string) bool {
	var confirmed bool
	confirm := huh.NewConfirm().
		Title(message).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed)

	runForm(confirm)
	return confirmed
}

// promptOverwrite asks whether an existing destination file should be
// overwritten. It never aborts the whole program: if the user aborts the
// prompt, it reports the file as skipped instead.
func promptOverwrite(dst string) bool {
	var overwrite bool
	confirm := huh.NewConfirm().
		Title(fmt.Sprintf("File %s already exists. Overwrite?", dst)).
		Affirmative("Yes").
		Negative("No").
		Value(&overwrite)

	err := huh.NewForm(huh.NewGroup(confirm)).WithShowHelp(true).Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			// Aborting the overwrite prompt skips only this file.
			return false
		}
		pterm.Error.Printfln("Prompt failed: %v", err)
		return false
	}
	return overwrite
}
