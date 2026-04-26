package config

import (
	"time"
)

// Policy defines the chaos injection rules for a specific target endpoint.
type Policy struct {
	Target          string        `yaml:"target"`
	Type            string        `yaml:"type"`
	LatencyChance   float64       `yaml:"latency_chance"`
	LatencyDuration time.Duration `yaml:"latency_duration"`
	ErrorChance     float64       `yaml:"error_chance"`
}

// PastaayConfig represents the root structure of the YAML configuration file.
type PastaayConfig struct {
	Version  int      `yaml:"version"`
	Policies []Policy `yaml:"policies"`
}
