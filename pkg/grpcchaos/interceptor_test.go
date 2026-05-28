package grpcchaos

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/CemAkan/pastaay/pkg/config"
)

// TestMatchMetadataPure verifies zero-allocation, case-insensitive metadata matching
func TestMatchMetadataPure(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-test-user": "true",
		"x-device":    "ios",
	})

	tests := []struct {
		name     string
		required map[string]string
		expected bool
	}{
		{"No requirements should match", nil, true},
		{"Empty requirements should match", map[string]string{}, true},
		{"Exact match", map[string]string{"x-test-user": "true"}, true},
		{"Case insensitive key match", map[string]string{"X-Test-User": "true"}, true},
		{"Value mismatch", map[string]string{"x-test-user": "false"}, false},
		{"Missing key", map[string]string{"x-version": "1.0"}, false},
		{"Multiple exact matches", map[string]string{"x-test-user": "true", "x-device": "ios"}, true},
		{"One match one miss should fail", map[string]string{"x-test-user": "true", "x-version": "1.0"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMetadataPure(md, tt.required); got != tt.expected {
				t.Errorf("matchMetadataPure() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestWaitContext_Timeout ensures that if a client drops the connection or timeout occurs,
// the delay is immediately aborted and prevents zombie goroutines (Memory Leak Prevention).
func TestWaitContext_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()

	// Simulate a 100ms chaos delay, but context will cancel in 10ms
	err := waitContext(ctx, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.DeadlineExceeded && st.Code() != codes.Canceled {
		t.Errorf("Expected context canceled or deadline exceeded status, got: %v", err)
	}

	// If elapsed is 50ms or more, our context abort logic is broken
	if elapsed >= 50*time.Millisecond {
		t.Errorf("waitContext did not abort early. Elapsed: %v", elapsed)
	}
}

// TestWaitContext_Success ensures that a normal delay completes successfully
func TestWaitContext_Success(t *testing.T) {
	ctx := context.Background()
	start := time.Now()

	err := waitContext(ctx, 10*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("waitContext returned too early. Elapsed: %v", elapsed)
	}
}

// TestGenerateError verifies that the policy correctly translates to gRPC Status Codes
func TestGenerateError(t *testing.T) {
	// 1. Default Fallback Test
	pDefault := config.Policy{}
	err1 := generateError(pDefault)
	st1, _ := status.FromError(err1)
	if st1.Code() != codes.Unavailable || st1.Message() != "Pastaay Chaos Injected" {
		t.Errorf("Default error generation failed: %v", err1)
	}

	// 2. Custom Payload Test
	pCustom := config.Policy{
		ErrorCode: int(codes.ResourceExhausted),
		ErrorBody: "Rate Limited",
	}
	err2 := generateError(pCustom)
	st2, _ := status.FromError(err2)
	if st2.Code() != codes.ResourceExhausted || st2.Message() != "Rate Limited" {
		t.Errorf("Custom error generation failed: %v", err2)
	}
}
func TestStreamCache_KeyedByHashNotName(t *testing.T) {
	a := config.Policy{Name: "duo", Type: "grpc", Target: "all", LatencyChance: 1.0, ErrorChance: 0, StreamRollMode: "stream"}
	b := config.Policy{Name: "duo", Type: "grpc", Target: "all", LatencyChance: 0, ErrorChance: 1.0, StreamRollMode: "stream"}

	mgr := config.NewManager(&config.PastaayConfig{
		Version:  1,
		Policies: []config.Policy{a, b},
	})
	got := mgr.GetActivePolicies("grpc")
	if len(got) != 2 {
		t.Fatalf("expected both policies to survive Update, got %d", len(got))
	}
	if got[0].PolicyHash == got[1].PolicyHash {
		t.Fatalf("policies with different rolls must have different PolicyHash; got %x", got[0].PolicyHash)
	}
	//keyed by hash, both policies fit.
	cache := make(map[uint64]PolicyDecision)
	cache[got[0].PolicyHash] = PolicyDecision{Hash: got[0].PolicyHash}
	cache[got[1].PolicyHash] = PolicyDecision{Hash: got[1].PolicyHash}
	if len(cache) != 2 {
		t.Fatalf("hash-keyed cache must hold both same-named policies, got %d", len(cache))
	}
}
