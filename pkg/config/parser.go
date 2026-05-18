package config

import (
	"log"
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

func decodeOrWarn(p *Policy, field string, node yaml.Node, dst interface{}) {
	if err := node.Decode(dst); err != nil {
		log.Printf("[WARN] policy %q: %s decode error: %v", p.Name, field, err)
	}
}
