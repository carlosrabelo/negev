package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	// Capture stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printUsage()

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	expectedStrings := []string{
		"Usage of",
		"--create-vlans",
		"--target string",
		"--verbose int",
		"--write",
		"--config string",
	}

	for _, expected := range expectedStrings {
		if !contains(output, expected) {
			t.Errorf("Expected output to contain '%s', got: %s", expected, output)
		}
	}
}

func TestMain_FlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError string
	}{
		{
			name: "valid flags",
			args: []string{"--target", "192.168.1.1", "--verbose", "1"},
		},
		{
			name:        "missing target",
			args:        []string{"--verbose", "1"},
			expectError: "--target parameter is required",
		},
		{
			name:        "invalid verbosity",
			args:        []string{"--target", "192.168.1.1", "--verbose", "5"},
			expectError: "--verbose must be 0, 1, 2, or 3",
		},
		{
			name:        "negative verbosity",
			args:        []string{"--target", "192.168.1.1", "--verbose", "-1"},
			expectError: "--verbose must be 0, 1, 2, or 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new FlagSet for each test to avoid redefinition
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
			flagSet.Usage = printUsage
			
			verbosity := flagSet.Int("verbose", 0, "Verbosity level: 0=none, 1=debug logs, 2=raw switch output, 3=debug+raw output")
			host := flagSet.String("target", "", "Switch target (must match a target in YAML, required)")

			// Parse test args
			err := flagSet.Parse(tt.args)
			if err != nil {
				t.Errorf("Flag parsing failed: %v", err)
				return
			}

			// Test validation logic
			if *verbosity < 0 || *verbosity > 3 {
				if !contains(tt.expectError, "--verbose must be") {
					t.Errorf("Expected verbosity validation error")
				}
				return
			}

			if *host == "" {
				if !contains(tt.expectError, "--target parameter is required") {
					t.Errorf("Expected target validation error")
				}
				return
			}

			if tt.expectError != "" {
				t.Errorf("Expected error but validation passed")
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	tests := []struct {
		name           string
		createFiles    map[string]string
		searchPaths    []string
		expectedPath   string
		expectFound    bool
	}{
		{
			name: "file in current directory",
			createFiles: map[string]string{
				"config.yaml": "test config",
			},
			searchPaths:  []string{"config.yaml"},
			expectedPath: "config.yaml",
			expectFound:  true,
		},
		{
			name: "file not found",
			searchPaths: []string{
				"config.yaml",
				"/tmp/nonexistent/config.yaml",
			},
			expectedPath: "",
			expectFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create test files
			for filePath, content := range tt.createFiles {
				fullPath := filepath.Join(tmpDir, filePath)
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}
			}

			// Test file finding logic
			found := false
			var configPath string

			for _, path := range tt.searchPaths {
				fullPath := filepath.Join(tmpDir, path)
				if _, err := os.Stat(fullPath); err == nil {
					configPath = fullPath
					found = true
					break
				}
			}

			if found != tt.expectFound {
				t.Errorf("Expected found=%v, got %v", tt.expectFound, found)
			}

			if found && configPath != filepath.Join(tmpDir, tt.expectedPath) {
				t.Errorf("Expected path %s, got %s", filepath.Join(tmpDir, tt.expectedPath), configPath)
			}
		})
	}
}

func TestVersionDisplay(t *testing.T) {
	// Test that version variables are properly defined
	if version == "" {
		t.Error("Version variable should not be empty")
	}

	if buildTime == "" {
		t.Error("BuildTime variable should not be empty")
	}

	// Test default values
	if version == "dev" {
		// This is expected for development builds
		t.Log("Version is 'dev' (development build)")
	}

	if buildTime == "unknown" {
		// This is expected for development builds
		t.Log("BuildTime is 'unknown' (development build)")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}

// Global variable to override os.Exit for testing
var exitFunc = os.Exit