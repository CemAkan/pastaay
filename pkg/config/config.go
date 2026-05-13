package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Policy struct {
	Name              string            `yaml:"name"`
	Target            string            `yaml:"target"`
	Type              string            `yaml:"type"`
	LatencyChance     float64           `yaml:"latency_chance"`
	LatencyDuration   time.Duration     `yaml:"latency_duration"`
	ErrorChance       float64           `yaml:"error_chance"`
	ErrorCode         int               `yaml:"error_code,omitempty"`
	ErrorBody         string            `yaml:"error_body,omitempty"`
	MatchHeaders      map[string]string `yaml:"match_headers,omitempty"`
	DropConnection    bool              `yaml:"drop_connection,omitempty"`
	StreamRollMode    string            `yaml:"stream_roll_mode,omitempty"`
	ThrottleThreshold int               `yaml:"throttle_threshold,omitempty"`
	RAMChunkMB        int               `yaml:"ram_chunk_mb,omitempty"`
	RAMInterval       time.Duration     `yaml:"ram_interval,omitempty"`
	CompiledRegex     *regexp.Regexp    `yaml:"-"`
	PolicyHash        uint64            `yaml:"-"`
	IsWildcard        bool              `yaml:"-"`
	WildcardPrefix    string            `yaml:"-"`
}

type PastaayConfig struct {
	Version              int                 `yaml:"version"`
	WarmupDuration       time.Duration       `yaml:"warmup_duration"`
	EnableDefaultIgnored bool                `yaml:"enable_default_ignored"`
	IgnoredCommands      map[string][]string `yaml:"ignored_commands"`
	Policies             []Policy            `yaml:"policies"`
}

var DefaultProtectedCommands = map[string][]string{
	"sql":      {"CREATE", "ALTER", "DROP", "TRUNCATE"},
	"mongo":    {"create", "createIndexes", "drop", "collMod"},
	"redis":    {"PING", "INFO", "CONFIG"},
	"grpc":     {"grpc.health.v1.Health"},
	"kafka":    {"__consumer_offsets", "_schemas", "__transaction_state"},
	"rabbitmq": {"amq.", "reply_"},
}

var validProtocols = map[string]bool{
	"http": true, "sql": true, "grpc": true, "redis": true,
	"mongo": true, "kafka": true, "rabbitmq": true, "resource": true,
}

// Validate performs strict bounds checking and protocol-specific sanity checks.
func (c *PastaayConfig) Validate() error {
	var errs []error

	if c.Version < 1 {
		errs = append(errs, errors.New("global: version must be at least 1"))
	}

	for i, p := range c.Policies {
		prefix := fmt.Sprintf("policy[%d] (%s):", i, p.Name)

		if p.Type == "" || !validProtocols[strings.ToLower(p.Type)] {
			errs = append(errs, fmt.Errorf("%s invalid or unsupported protocol", prefix))
		}
		if p.Target == "" {
			errs = append(errs, fmt.Errorf("%s target cannot be empty", prefix))
		}

		// Logical Sanity
		if p.LatencyChance < 0 || p.LatencyChance > 1.0 {
			errs = append(errs, fmt.Errorf("%s latency_chance must be between 0.0 and 1.0", prefix))
		}
		if p.ErrorChance < 0 || p.ErrorChance > 1.0 {
			errs = append(errs, fmt.Errorf("%s error_chance must be between 0.0 and 1.0", prefix))
		}
		if p.LatencyDuration < 0 {
			errs = append(errs, fmt.Errorf("%s latency_duration cannot be negative", prefix))
		}
		if p.RAMChunkMB < 0 {
			errs = append(errs, fmt.Errorf("%s ram_chunk_mb cannot be negative", prefix))
		}
		if p.ThrottleThreshold < 0 {
			errs = append(errs, fmt.Errorf("%s throttle_threshold cannot be negative", prefix))
		}

		switch strings.ToLower(p.Type) {
		case "http":
			if p.ErrorCode != 0 && (p.ErrorCode < 100 || p.ErrorCode > 599) {
				errs = append(errs, fmt.Errorf("%s invalid HTTP status code: %d", prefix, p.ErrorCode))
			}
		case "grpc":
			if p.ErrorCode < 0 || p.ErrorCode > 16 {
				errs = append(errs, fmt.Errorf("%s invalid gRPC status code: %d", prefix, p.ErrorCode))
			}
		case "sql":
			if !strings.EqualFold(p.Target, "all") && !strings.EqualFold(p.Target, "database") {
				if _, err := regexp.Compile(p.Target); err != nil {
					errs = append(errs, fmt.Errorf("%s invalid regex pattern: %w", prefix, err))
				}
			}
		}
	}

	return errors.Join(errs...)
}
