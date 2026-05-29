package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// maxConfigFileBytes protects against OOM from a hostile/accidental gigabyte config file.
const maxConfigFileBytes = 5 << 20 // 5 MiB

// LoadConfig reads, sanitizes, and validates the YAML configuration file.
func LoadConfig(filePath string) (*PastaayConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	file, err := io.ReadAll(io.LimitReader(f, maxConfigFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(file)) > maxConfigFileBytes {
		return nil, fmt.Errorf("config file %s exceeds %d byte ceiling", filePath, maxConfigFileBytes)
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
