package tracing

import (
	"context"
	"testing"
)

// TestNoopFallback ensures that when the provider is NOT initialized
func TestNoopFallback(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize with empty endpoint (Simulates OTel disabled)
	shutdown, err := InitProvider(ctx, "")
	if err != nil {
		t.Fatalf("Expected no error for empty endpoint, got %v", err)
	}
	defer shutdown(ctx)

	// 2. Start a span
	spanCtx, span := StartChaosSpan(ctx, "pastaay.test", "all", "latency")

	// 3. Verify context is still valid and span doesn't panic on End()
	if spanCtx == nil {
		t.Fatal("Expected valid context, got nil")
	}

	// If this panics or blocks, the No-Op fallback is broken.
	span.End()
}
