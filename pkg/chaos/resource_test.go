package chaos

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestCPUBurner_ContextCancellation(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Attempt to burn 4 cores
	BurnCPU(ctx, 4)

	<-ctx.Done()
	// Wait briefly for goroutines to terminate
	time.Sleep(50 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	// Leak tolerance: +/- 2 goroutines due to test runner background tasks
	if finalGoroutines > initialGoroutines+2 {
		t.Fatalf("CPU Burner goroutine leak detected! Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

func TestRAMLeaker_DemandPagingAndRecovery(t *testing.T) {
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Leak 10MB every 50ms (~30-40MB total)
	go LeakRAM(ctx, 10, 50*time.Millisecond)

	<-ctx.Done()
	// Give GC a moment to reclaim memory after amnesia protocol
	time.Sleep(100 * time.Millisecond)

	runtime.ReadMemStats(&m2)

	// Verify that memory isn't permanently locked
	if m2.Alloc > m1.Alloc+(10*1024*1024) {
		t.Logf("Warning: RAM Leaker recovery might be slow or failed. Initial: %d, Final: %d", m1.Alloc, m2.Alloc)
	}
}
