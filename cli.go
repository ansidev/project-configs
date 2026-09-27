package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	// defaultConfigFile is the config file read when --config-file is omitted.
	defaultConfigFile = "config.yaml"
	// defaultConfigDir is the source directory read when --config-dir is
	// omitted.
	defaultConfigDir = "./configs"
)

// appOptions holds the resolved and validated settings for a single CLI run.
type appOptions struct {
	// help reports whether the user asked for the usage text.
	help bool
	// configFile is the path of the config YAML file to load.
	configFile string
	// configDir is the directory that contains the source configuration files.
	configDir string
}

// parseOptions parses the given command line arguments into appOptions.
//
// Both path flags are normalized and the config directory is verified to exist
// before returning, so an unusable override fails here rather than after the
// user has answered every prompt. Usage errors are returned to the caller
// instead of being printed, because the FlagSet output is discarded.
func parseOptions(args []string) (appOptions, error) {
	var opts appOptions

	fs := flag.NewFlagSet("project-configs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.help, "help", false, "Display help information")
	fs.StringVar(&opts.configFile, "c", defaultConfigFile, "Path to the config YAML file")
	fs.StringVar(&opts.configFile, "config-file", defaultConfigFile, "Path to the config YAML file")
	fs.StringVar(&opts.configDir, "d", defaultConfigDir, "Path to the directory that contains the source configuration files")
	fs.StringVar(&opts.configDir, "config-dir", defaultConfigDir, "Path to the directory that contains the source configuration files")

	if err := fs.Parse(args); err != nil {
		return appOptions{}, err
	}

	// The usage text is the whole answer for --help, so skip validation.
	if opts.help {
		return opts, nil
	}

	var err error
	if opts.configFile, err = normalizePathFlag("config-file", opts.configFile); err != nil {
		return appOptions{}, err
	}
	if opts.configDir, err = normalizePathFlag("config-dir", opts.configDir); err != nil {
		return appOptions{}, err
	}
	if err := validateConfigDir(opts.configDir); err != nil {
		return appOptions{}, err
	}

	return opts, nil
}

// normalizePathFlag normalizes a path supplied through a CLI flag, naming the
// flag in any validation error so the user knows which input to fix.
func normalizePathFlag(name, value string) (string, error) {
	normalized, err := convertToFilePath(value)
	if err != nil {
		return "", fmt.Errorf("invalid --%s: %w", name, err)
	}
	return normalized, nil
}

// validateConfigDir ensures the configuration source directory exists and is a
// directory, so a mistyped override is reported before any copying starts.
func validateConfigDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("invalid --config-dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid --config-dir: %s is not a directory", dir)
	}
	return nil
}
