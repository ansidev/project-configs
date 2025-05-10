package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pterm/pterm"
)

const BASE_SOURCE_DIR = "./configs"

type CopyEvent struct {
	SrcPath     string
	DestPath    string
	PostMessage string
	Error       error
}

// CopyManager holds the state for file copying operations
type CopyManager struct {
	confirmMu sync.Mutex
}

// NewCopyManager creates a new CopyManager with initialized fields
func NewCopyManager() *CopyManager {
	return &CopyManager{
		confirmMu: sync.Mutex{},
	}
}

func (cm *CopyManager) copyFile(baseSrcDir string, src configMetadata, dst string, eventChan chan<- CopyEvent) {
	// Open source file
	srcFile, err := os.Open(filepath.Join(baseSrcDir, src.Path))
	if err != nil {
		eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: err}
		return
	}
	defer srcFile.Close()

	// Get the directory path from the input (removes filename if present)
	dir := filepath.Dir(dst)

	// Create directories recursively
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: fmt.Errorf("failed to create directory path %s: %v", dir, err)}
		return
	}

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: err}
		return
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: err}
		return
	}

	// Ensure file is written to disk
	err = dstFile.Sync()
	if err != nil {
		eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: err}
		return
	}

	eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: nil, PostMessage: src.PostMessage}
}

func (cm *CopyManager) CopyFilesConcurrently(srcFiles []configMetadata, destDir string) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	eventChan := make(chan CopyEvent, len(srcFiles))
	var wg sync.WaitGroup

	// Process each file
	for _, src := range srcFiles {
		wg.Add(1)
		dst := filepath.Join(destDir, src.Path)

		go func(src configMetadata, dst string) {
			defer wg.Done()

			// Check if destination file exists
			if _, err := os.Stat(dst); err == nil {
				// Lock for confirmation
				cm.confirmMu.Lock()
				result, _ := pterm.DefaultInteractiveConfirm.
					WithDefaultText(fmt.Sprintf("File %s already exists. Overwrite?", dst)).
					Show()
				cm.confirmMu.Unlock()

				if !result {
					eventChan <- CopyEvent{SrcPath: src.Path, DestPath: dst, Error: fmt.Errorf("overwrite cancelled by user")}
					return
				}
			}

			// Proceed with copying
			cm.copyFile(BASE_SOURCE_DIR, src, dst, eventChan)
		}(src, dst)
	}

	// Close event channel when all copying is done
	go func() {
		wg.Wait()
		close(eventChan)
	}()

	return cm.processCopyEvents(eventChan)
}

// processCopyEvents process copy events
func (cm *CopyManager) processCopyEvents(eventChan chan CopyEvent) error {
	successCount := 0
	errorCount := 0
	var successEvents []CopyEvent

	for event := range eventChan {
		// Lock to ensure no active confirmation prompt is in progress
		cm.confirmMu.Lock()
		if event.Error != nil {
			errorCount++
			pterm.Error.Printfln("Failed to copy %s to %s: %v",
				event.SrcPath, event.DestPath, event.Error)

		} else {
			successCount++
			pterm.Success.Printfln("Successfully copied %s to %s",
				event.SrcPath, event.DestPath)
			if len(event.PostMessage) > 0 {
				successEvents = append(successEvents, event)
			}
		}
		cm.confirmMu.Unlock()
	}

	var result error

	pterm.Println()

	if errorCount > 0 {
		pterm.Error.Printfln("Completed with %d errors", errorCount)
		result = fmt.Errorf("%d files failed to copy", errorCount)
	} else {
		pterm.Success.Printfln("Successfully copied %d files", successCount)
	}

	pterm.Println()

	for _, successEvent := range successEvents {
		pterm.Info.Printfln("%s: %s", successEvent.DestPath, successEvent.PostMessage)
	}

	return result
}
