package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
)

// TestResponse_JSONSerialization verifies that our API response struct
// correctly serializes to JSON with the proper tags.
func TestResponse_JSONSerialization(t *testing.T) {
	resp := Response{
		Message: "URL successfully shortened and saved to database!",
		Short:   "http://short.ly/xyz123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	jsonString := string(data)
	if !strings.Contains(jsonString, `"message":"URL successfully shortened`) {
		t.Errorf("Expected JSON to contain correct message, got: %s", jsonString)
	}
}

// TestDemoConfigParsing verifies that the isolated demo config loads correctly.
func TestDemoConfigParsing(t *testing.T) {
	cfg, err := config.LoadConfig("pastaay.yaml")
	if err != nil {
		t.Fatalf("Failed to load demo pastaay.yaml: %v", err)
	}

	if len(cfg.Policies) == 0 {
		t.Errorf("Expected policies in demo config, got 0")
	}
}
