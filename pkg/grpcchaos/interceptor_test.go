package grpcchaos

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/CemAkan/pastaay/pkg/config"
)

func TestMatchMetadata(t *testing.T) {
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
		{"No requirements should match", nil, true},
		{"Single matching requirement", map[string]string{"x-test-user": "true"}, true},
		{"Mismatching requirement", map[string]string{"x-test-user": "false"}, false},
		{"Missing required metadata", map[string]string{"x-beta-flag": "true"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMetadata(ctx, tt.required); got != tt.expected {
				t.Errorf("matchMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// mockServerStream implements grpc.ServerStream for testing purposes
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestStreamInterceptorBypass(t *testing.T) {
	cfgManager := config.NewManager(&config.PastaayConfig{})
	interceptor := StreamInterceptor(cfgManager)

	mockStream := &mockServerStream{
		ctx: context.Background(),
	}

	info := &grpc.StreamServerInfo{
		FullMethod: "/test.v1.Service/StreamMethod",
	}

	handlerCalled := false
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	err := interceptor(nil, mockStream, info, handler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !handlerCalled {
		t.Errorf("Expected stream handler to be called")
	}
}
