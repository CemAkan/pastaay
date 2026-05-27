package config

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Policy defines a single chaos injection rule. Every field supports both snake_case (YAML) and camelCase (JSON/K8s) keys via custom UnmarshalYAML.
type Policy struct {
	Name              string            `yaml:"name" json:"name"`
	Target            string            `yaml:"target" json:"target"`
	Type              string            `yaml:"type" json:"type"`
	LatencyChance     float64           `yaml:"latency_chance" json:"latencyChance"`
	LatencyDuration   time.Duration     `yaml:"latency_duration" json:"latencyDuration"`
	ErrorChance       float64           `yaml:"error_chance" json:"errorChance"`
	ErrorCode         int               `yaml:"error_code,omitempty" json:"errorCode,omitempty"`
	ErrorBody         string            `yaml:"error_body,omitempty" json:"errorBody,omitempty"`
	MatchHeaders      map[string]string `yaml:"match_headers,omitempty" json:"matchHeaders,omitempty"`
	DropConnection    bool              `yaml:"drop_connection,omitempty" json:"dropConnection,omitempty"`
	StreamRollMode    string            `yaml:"stream_roll_mode,omitempty" json:"streamRollMode,omitempty"`
	ThrottleThreshold int               `yaml:"throttle_threshold,omitempty" json:"throttleThreshold,omitempty"`
	RAMChunkMB        int               `yaml:"ram_chunk_mb,omitempty" json:"ramChunkMB,omitempty"`
	RAMInterval       time.Duration     `yaml:"ram_interval,omitempty" json:"ramInterval,omitempty"`
	CompiledRegex     *regexp.Regexp    `yaml:"-" json:"-"`
	PolicyHash        uint64            `yaml:"-" json:"-"`
	MetricTag         string            `yaml:"-" json:"-"`
	IsWildcard        bool              `yaml:"-" json:"-"`
	WildcardPrefix    string            `yaml:"-" json:"-"`
}

// configDecodeOrWarn decodes a generic camelCase scalar into target and logs warnings instead of silently dropping bad values.
func configDecodeOrWarn(node yaml.Node, fieldName string, target interface{}) {
	if err := node.Decode(target); err != nil {
		log.Printf("[Pastaay-Config] WARN: field %q decode failed: %v", fieldName, err)
	}
}

// configDecodeDuration parses a string node as time.Duration, logging both type errors and parse errors instead of swallowing them.
func configDecodeDuration(node yaml.Node, fieldName string, target *time.Duration) {
	if *target != 0 {
		return
	}
	var s string
	if err := node.Decode(&s); err != nil {
		log.Printf("[Pastaay-Config] WARN: field %q is not a string: %v", fieldName, err)
		return
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("[Pastaay-Config] WARN: invalid duration for %q (%q): %v", fieldName, s, err)
		return
	}
	*target = d
}

// UnmarshalYAML implements custom dual-casing support to capture both snake_case and camelCase parameters.
func (p *Policy) UnmarshalYAML(value *yaml.Node) error {
	type shadowPolicy Policy
	var s shadowPolicy
	// First, parse standard tags (snake_case)
	if err := value.Decode(&s); err != nil {
		return err
	}
	*p = Policy(s)

	// Decode into a raw node map to dynamically catch camelCase fallbacks from K8s/JSON streams.
	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		return nil // Non-map payloads are safely bypassed
	}

	if node, ok := rawMap["latencyChance"]; ok && p.LatencyChance == 0 {
		configDecodeOrWarn(node, "latencyChance", &p.LatencyChance)
	}
	if node, ok := rawMap["latencyDuration"]; ok {
		configDecodeDuration(node, "latencyDuration", &p.LatencyDuration)
	}
	if node, ok := rawMap["errorChance"]; ok && p.ErrorChance == 0 {
		configDecodeOrWarn(node, "errorChance", &p.ErrorChance)
	}
	if node, ok := rawMap["errorCode"]; ok && p.ErrorCode == 0 {
		configDecodeOrWarn(node, "errorCode", &p.ErrorCode)
	}
	if node, ok := rawMap["errorBody"]; ok && p.ErrorBody == "" {
		configDecodeOrWarn(node, "errorBody", &p.ErrorBody)
	}
	if node, ok := rawMap["matchHeaders"]; ok && len(p.MatchHeaders) == 0 {
		configDecodeOrWarn(node, "matchHeaders", &p.MatchHeaders)
	}
	if node, ok := rawMap["dropConnection"]; ok && !p.DropConnection {
		configDecodeOrWarn(node, "dropConnection", &p.DropConnection)
	}
	if node, ok := rawMap["streamRollMode"]; ok && p.StreamRollMode == "" {
		configDecodeOrWarn(node, "streamRollMode", &p.StreamRollMode)
	}
	if node, ok := rawMap["throttleThreshold"]; ok && p.ThrottleThreshold == 0 {
		configDecodeOrWarn(node, "throttleThreshold", &p.ThrottleThreshold)
	}
	if node, ok := rawMap["ramChunkMB"]; ok && p.RAMChunkMB == 0 {
		configDecodeOrWarn(node, "ramChunkMB", &p.RAMChunkMB)
	}
	if node, ok := rawMap["ramInterval"]; ok {
		configDecodeDuration(node, "ramInterval", &p.RAMInterval)
	}

	return nil
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
