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

const (
	maxConfigVersion    = 1
	maxLatencyDuration  = 1 * time.Hour
	maxRAMInterval      = 1 * time.Hour
	maxThrottleCeiling  = 10_000_000
	maxRAMChunkPerEntry = 4096
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

func configDecodeOrWarn(node yaml.Node, fieldName string, target interface{}) {
	if err := node.Decode(target); err != nil {
		log.Printf("[Pastaay-Config] WARN: field %q decode failed: %v", fieldName, err)
	}
}

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
	if err := value.Decode(&s); err != nil {
		return err
	}
	*p = Policy(s)

	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		if value.Kind != yaml.MappingNode {
			return nil
		}
		return fmt.Errorf("policy decode rawmap: %w", err)
	}

	present := func(snake string) bool {
		_, ok := rawMap[snake]
		return ok
	}

	if node, ok := rawMap["latencyChance"]; ok && !present("latency_chance") {
		configDecodeOrWarn(node, "latencyChance", &p.LatencyChance)
	}
	if node, ok := rawMap["latencyDuration"]; ok && !present("latency_duration") {
		configDecodeDuration(node, "latencyDuration", &p.LatencyDuration)
	}
	if node, ok := rawMap["errorChance"]; ok && !present("error_chance") {
		configDecodeOrWarn(node, "errorChance", &p.ErrorChance)
	}
	if node, ok := rawMap["errorCode"]; ok && !present("error_code") {
		configDecodeOrWarn(node, "errorCode", &p.ErrorCode)
	}
	if node, ok := rawMap["errorBody"]; ok && !present("error_body") {
		configDecodeOrWarn(node, "errorBody", &p.ErrorBody)
	}
	if node, ok := rawMap["matchHeaders"]; ok && !present("match_headers") {
		configDecodeOrWarn(node, "matchHeaders", &p.MatchHeaders)
	}
	if node, ok := rawMap["dropConnection"]; ok && !present("drop_connection") {
		configDecodeOrWarn(node, "dropConnection", &p.DropConnection)
	}
	if node, ok := rawMap["streamRollMode"]; ok && !present("stream_roll_mode") {
		configDecodeOrWarn(node, "streamRollMode", &p.StreamRollMode)
	}
	if node, ok := rawMap["throttleThreshold"]; ok && !present("throttle_threshold") {
		configDecodeOrWarn(node, "throttleThreshold", &p.ThrottleThreshold)
	}
	if node, ok := rawMap["ramChunkMB"]; ok && !present("ram_chunk_mb") {
		configDecodeOrWarn(node, "ramChunkMB", &p.RAMChunkMB)
	}
	if node, ok := rawMap["ramInterval"]; ok && !present("ram_interval") {
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

var validStreamRollModes = map[string]bool{
	"":          true, // default
	"per-call":  true,
	"per-frame": true,
	"stream":    true,
}

// Validate performs strict bounds checking and protocol-specific sanity checks.
func (c *PastaayConfig) Validate() error {
	var errs []error

	if c.Version < 1 || c.Version > maxConfigVersion {
		errs = append(errs, fmt.Errorf("global: version must be between 1 and %d", maxConfigVersion))
	}
	if c.WarmupDuration < 0 {
		errs = append(errs, errors.New("global: warmup_duration cannot be negative"))
	}
	if c.WarmupDuration > 24*time.Hour {
		errs = append(errs, errors.New("global: warmup_duration unreasonably large (>24h)"))
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
		if p.LatencyDuration > maxLatencyDuration {
			errs = append(errs, fmt.Errorf("%s latency_duration exceeds ceiling %s", prefix, maxLatencyDuration))
		}
		if p.RAMInterval < 0 {
			errs = append(errs, fmt.Errorf("%s ram_interval cannot be negative", prefix))
		}
		if p.RAMInterval > maxRAMInterval {
			errs = append(errs, fmt.Errorf("%s ram_interval exceeds ceiling %s", prefix, maxRAMInterval))
		}
		if p.RAMChunkMB < 0 {
			errs = append(errs, fmt.Errorf("%s ram_chunk_mb cannot be negative", prefix))
		}
		if p.RAMChunkMB > maxRAMChunkPerEntry {
			errs = append(errs, fmt.Errorf("%s ram_chunk_mb exceeds ceiling %d", prefix, maxRAMChunkPerEntry))
		}
		if p.ThrottleThreshold < 0 {
			errs = append(errs, fmt.Errorf("%s throttle_threshold cannot be negative", prefix))
		}
		if p.ThrottleThreshold > maxThrottleCeiling {
			errs = append(errs, fmt.Errorf("%s throttle_threshold exceeds ceiling %d", prefix, maxThrottleCeiling))
		}
		if !validStreamRollModes[strings.ToLower(p.StreamRollMode)] {
			errs = append(errs, fmt.Errorf("%s invalid stream_roll_mode %q", prefix, p.StreamRollMode))
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
