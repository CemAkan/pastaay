package config

import (
	"time"
)

// Policy defines the chaos injection rules and target criteria.
type Policy struct {
	Name            string            `yaml:"name"`
	Target          string            `yaml:"target"`
	Type            string            `yaml:"type"`
	LatencyChance   float64           `yaml:"latency_chance"`
	LatencyDuration time.Duration     `yaml:"latency_duration"`
	ErrorChance     float64           `yaml:"error_chance"`
	ErrorCode       int               `yaml:"error_code,omitempty"`
	ErrorBody       string            `yaml:"error_body,omitempty"`
	MatchHeaders    map[string]string `yaml:"match_headers,omitempty"`
}

// PastaayConfig represents the root structure of the YAML configuration file.
type PastaayConfig struct {
	Version  int      `yaml:"version"`
	Policies []Policy `yaml:"policies"`
}
