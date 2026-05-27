package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// maxConfigFileBytes protects against OOM from a hostile/accidental gigabyte config file.
const maxConfigFileBytes = 5 << 20 // 5 MiB

// LoadConfig reads, sanitizes, and validates the YAML configuration file.
func LoadConfig(filePath string) (*PastaayConfig, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if fi.Size() > maxConfigFileBytes {
		return nil, fmt.Errorf("config file %s is %d bytes — exceeds limit of %d", filePath, fi.Size(), maxConfigFileBytes)
	}

	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if hasSuspiciousYAMLAlias(file) {
		return nil, fmt.Errorf("config file %s: YAML aliases are not permitted (potential alias-bomb)", filePath)
	}

	var cfg PastaayConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %s validation: %w", filePath, err)
	}

	return &cfg, nil
}
