package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses the YAML configuration file from the given path.
func LoadConfig(filePath string) (*PastaayConfig, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg PastaayConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
