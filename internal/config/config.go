package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// Config represents the application settings.
type Config struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
	FontSize int    `json:"font_size"`
}

var currentConfig Config

// Default returns a configuration with sensible defaults.
func Default() Config {
	return Config{
		Language: "en",
		Theme:    "dark",
		FontSize: 16,
	}
}

// Get returns the current configuration.
func Get() Config {
	return currentConfig
}

// Set updates the current configuration in memory.
func Set(c Config) {
	currentConfig = c
}

// configFilePath returns the absolute path to the config.json file.
func configFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// Load reads the configuration from disk. If the file does not exist, it loads defaults.
func Load() {
	currentConfig = Default()
	path := configFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("Config file not found, using defaults", "path", path)
		} else {
			slog.Error("Failed to read config file", "error", err)
		}
		return
	}

	if err := json.Unmarshal(data, &currentConfig); err != nil {
		slog.Error("Failed to parse config file, using defaults", "error", err)
	} else {
		slog.Info("Config loaded successfully", "path", path)
	}
}

// Save writes the current configuration to disk.
func Save() error {
	path := configFilePath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Failed to create config directory", "error", err)
		return err
	}

	data, err := json.MarshalIndent(currentConfig, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal config", "error", err)
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("Failed to write config file", "error", err)
		return err
	}

	slog.Info("Config saved successfully", "path", path)
	return nil
}
