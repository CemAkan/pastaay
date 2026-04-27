package grpcchaos

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestMatchMetadata(t *testing.T) {
	// Create a mock context with some metadata
	// Note: gRPC metadata keys are automatically converted to lowercase
	md := metadata.New(map[string]string{
		"x-test-user": "true",
		"x-device":    "ios",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	tests := []struct {
		name     string
		required map[string]string
		expected bool
	}{
		{
			name:     "No requirements should match",
			required: nil,
			expected: true,
		},
		{
			name:     "Single matching requirement",
			required: map[string]string{"x-test-user": "true"},
			expected: true,
		},
		{
			name:     "Mismatching requirement",
			required: map[string]string{"x-test-user": "false"},
			expected: false,
		},
		{
			name:     "Missing required metadata",
			required: map[string]string{"x-beta-flag": "true"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMetadata(ctx, tt.required); got != tt.expected {
				t.Errorf("matchMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}
