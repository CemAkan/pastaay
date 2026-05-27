package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// maxConfigFileBytes protects against OOM from a hostile/accidental gigabyte config file.
const maxConfigFileBytes = 5 << 20 // 5 MiB

// LoadConfig reads and parses the YAML configuration file from the given path.
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

	var cfg PastaayConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func decodeOrWarn(p *Policy, field string, node yaml.Node, dst interface{}) {
	if err := node.Decode(dst); err != nil {
		log.Printf("[WARN] policy %q: %s decode error: %v", p.Name, field, err)
	}
}
