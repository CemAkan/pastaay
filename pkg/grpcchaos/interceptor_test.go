package grpcchaos

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestMatchMetadata(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-test-user": "true",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	tests := []struct {
		name     string
		required map[string]string
		expected bool
	}{
		{"Matching requirement", map[string]string{"x-test-user": "true"}, true},
		{"Mismatching requirement", map[string]string{"x-test-user": "false"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMetadata(ctx, tt.required); got != tt.expected {
				t.Errorf("matchMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}
