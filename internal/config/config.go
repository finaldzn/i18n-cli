package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the configuration for the i18n-cli tool
type Config struct {
	// Source language to translate from
	SourceLang string `json:"sourceLang"`

	// Target languages to translate to
	TargetLangs []string `json:"targetLangs"`

	// Files to include (glob patterns)
	IncludeFiles []string `json:"includeFiles"`

	// Files to exclude (glob patterns)
	ExcludeFiles []string `json:"excludeFiles"`

	// OpenAI API key (can be overridden by environment variable)
	APIKey string `json:"apiKey"`

	// Batch size for translations (0 = one at a time)
	BatchSize int `json:"batchSize"`

	// Translation mode (full or missing)
	Mode string `json:"mode"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		SourceLang:   "en",
		TargetLangs:  []string{},
		IncludeFiles: []string{"*.json"},
		ExcludeFiles: []string{},
		BatchSize:    5,
		Mode:         "missing",
	}
}

// LoadConfig loads a configuration file
func LoadConfig(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file %s does not exist", path)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults for any missing fields
	if config.SourceLang == "" {
		config.SourceLang = "en"
	}

	if config.Mode == "" {
		config.Mode = "missing"
	}

	if len(config.IncludeFiles) == 0 {
		config.IncludeFiles = []string{"*.json"}
	}

	return &config, nil
}

// SaveConfig saves a configuration file
func SaveConfig(config *Config, path string) error {
	// Marshal JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write file
	return os.WriteFile(path, data, 0644)
}

// CreateDefaultConfig creates a default configuration file if it doesn't exist
func CreateDefaultConfig(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		// File exists, don't overwrite it
		return nil
	}

	// Create default config
	config := DefaultConfig()

	// Save config
	return SaveConfig(config, path)
}
