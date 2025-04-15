package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func FindPath(yamlFile string, verbosity int) (string, error) {
	if yamlFile != "config.yaml" {
		return yamlFile, nil
	}

	possiblePaths := []string{
		filepath.Join(".", "config.yaml"),
	}

	if runtime.GOOS == "linux" {
		if userConfigDir, err := os.UserConfigDir(); err == nil {
			possiblePaths = append(possiblePaths, filepath.Join(userConfigDir, "negev", "config.yaml"))
		}
		possiblePaths = append(possiblePaths, "/etc/negev/config.yaml")
	} else if runtime.GOOS == "windows" {
		if appDataDir := os.Getenv("APPDATA"); appDataDir != "" {
			possiblePaths = append(possiblePaths, filepath.Join(appDataDir, "negev", "config.yaml"))
		}
		if programDataDir := os.Getenv("ProgramData"); programDataDir != "" {
			possiblePaths = append(possiblePaths, filepath.Join(programDataDir, "negev", "config.yaml"))
		}
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			debugf(verbosity == 1 || verbosity == 3, "DEBUG: Configuration file found at %s\n", path)
			return path, nil
		}
	}

	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("no config.yaml file found in ./, %%APPDATA%%\\negev\\, or %%ProgramData%%\\negev\\")
	}
	return "", fmt.Errorf("no config.yaml file found in ./, ~/.config/negev/, or /etc/negev/")
}
