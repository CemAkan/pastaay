package main

import (
	"encoding/json"
	"strings"
	"testing"
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

	if !strings.Contains(jsonString, `"short_url":"http://short.ly/xyz123"`) {
		t.Errorf("Expected JSON to contain correct short_url, got: %s", jsonString)
	}
}
