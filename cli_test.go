package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsDefaults(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions() unexpected error: %v", err)
	}

	if opts.help {
		t.Error("parseOptions() help = true, want false")
	}
	if opts.configFile != defaultConfigFile {
		t.Errorf("parseOptions() configFile = %q, want %q", opts.configFile, defaultConfigFile)
	}
	// The default is normalized like any other value, so "./configs" becomes
	// "configs". Both refer to the same directory.
	if want := filepath.Clean(defaultConfigDir); opts.configDir != want {
		t.Errorf("parseOptions() configDir = %q, want %q", opts.configDir, want)
	}
}

func TestParseOptions(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "my config.yaml")
	otherDir := t.TempDir()
	trailingSlashDir := t.TempDir() + string(filepath.Separator)
	// Contains "." and a "nested/.." pair, both of which filepath.Clean removes.
	uncleanFile := dir + string(filepath.Separator) + "." + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "unused.yaml"

	tests := []struct {
		name           string
		args           []string
		wantConfigFile string
		wantConfigDir  string
	}{
		{
			name:           "config file is overridden with the long flag",
			args:           []string{"--config-file", configFile},
			wantConfigFile: configFile,
			wantConfigDir:  filepath.Clean(defaultConfigDir),
		},
		{
			name:           "config dir is overridden with the long flag",
			args:           []string{"--config-dir", otherDir},
			wantConfigFile: defaultConfigFile,
			wantConfigDir:  otherDir,
		},
		{
			name:           "flags accept the equals form",
			args:           []string{"--config-file=" + configFile, "--config-dir=" + otherDir},
			wantConfigFile: configFile,
			wantConfigDir:  otherDir,
		},
		{
			name:           "config file is overridden with the short flag",
			args:           []string{"-c", configFile},
			wantConfigFile: configFile,
			wantConfigDir:  filepath.Clean(defaultConfigDir),
		},
		{
			name:           "config dir is overridden with the short flag",
			args:           []string{"-d", otherDir},
			wantConfigFile: defaultConfigFile,
			wantConfigDir:  otherDir,
		},
		{
			name:           "both flags are overridden independently",
			args:           []string{"-c", configFile, "-d", otherDir},
			wantConfigFile: configFile,
			wantConfigDir:  otherDir,
		},
		{
			name:           "trailing separator is normalized away",
			args:           []string{"--config-dir", trailingSlashDir},
			wantConfigFile: defaultConfigFile,
			wantConfigDir:  filepath.Clean(trailingSlashDir),
		},
		{
			name:           "surrounding quotes from a shell paste are trimmed",
			args:           []string{"--config-dir", "'" + otherDir + "'"},
			wantConfigFile: defaultConfigFile,
			wantConfigDir:  otherDir,
		},
		{
			name:           "redundant path segments are cleaned",
			args:           []string{"--config-file", uncleanFile},
			wantConfigFile: filepath.Join(dir, "unused.yaml"),
			wantConfigDir:  filepath.Clean(defaultConfigDir),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseOptions(tt.args)
			if err != nil {
				t.Fatalf("parseOptions() unexpected error: %v", err)
			}
			if opts.configFile != tt.wantConfigFile {
				t.Errorf("parseOptions() configFile = %q, want %q", opts.configFile, tt.wantConfigFile)
			}
			if opts.configDir != tt.wantConfigDir {
				t.Errorf("parseOptions() configDir = %q, want %q", opts.configDir, tt.wantConfigDir)
			}
		})
	}
}

func TestParseOptionsErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown flag is rejected",
			args:    []string{"-x"},
			wantErr: "not defined",
		},
		{
			name:    "empty config file is rejected",
			args:    []string{"--config-file="},
			wantErr: "invalid --config-file",
		},
		{
			name:    "whitespace-only config file is rejected",
			args:    []string{"--config-file", "   "},
			wantErr: "invalid --config-file",
		},
		{
			name:    "empty config dir is rejected",
			args:    []string{"--config-dir="},
			wantErr: "invalid --config-dir",
		},
		{
			name:    "missing config dir is rejected",
			args:    []string{"--config-dir", filepath.Join(t.TempDir(), "does-not-exist")},
			wantErr: "invalid --config-dir",
		},
		{
			name:    "config dir pointing at a file is rejected",
			args:    []string{"--config-dir", defaultConfigFile},
			wantErr: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseOptions(tt.args)
			if err == nil {
				t.Fatalf("parseOptions() expected error for args %v, got %+v", tt.args, opts)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseOptions() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseOptionsHelpSkipsValidation(t *testing.T) {
	// --help must report the usage text even when the other flags are unusable.
	opts, err := parseOptions([]string{"--help", "--config-dir", filepath.Join(t.TempDir(), "does-not-exist")})
	if err != nil {
		t.Fatalf("parseOptions() unexpected error for --help: %v", err)
	}
	if !opts.help {
		t.Error("parseOptions() help = false, want true")
	}
}

func TestParseOptionsHelp(t *testing.T) {
	opts, err := parseOptions([]string{"--help"})
	if err != nil {
		t.Fatalf("parseOptions() unexpected error: %v", err)
	}
	if !opts.help {
		t.Error("parseOptions() help = false, want true")
	}
}

func TestValidateConfigDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{name: "an existing directory is accepted", dir: dir},
		{name: "a missing path is rejected", dir: filepath.Join(dir, "missing"), wantErr: true},
		{name: "a regular file is rejected", dir: file, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigDir(tt.dir)
			if tt.wantErr && err == nil {
				t.Error("validateConfigDir() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateConfigDir() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateConfigDirErrorNamesThePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	err := validateConfigDir(missing)
	if err == nil {
		t.Fatal("validateConfigDir() expected error for a missing path")
	}
	if !strings.Contains(err.Error(), "invalid --config-dir") {
		t.Errorf("validateConfigDir() error = %q, want it to name the flag", err.Error())
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("validateConfigDir() error = %q, want it to contain the path %q", err.Error(), missing)
	}
}
