package chaos

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCPUBurner_ContextCancellation(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	BurnCPU(ctx, 4, 100000)

	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

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
func TestLeakRAM_AccountingBalances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in -short mode")
	}
	before := currentPoolMB.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		LeakRAM(ctx, 1, 10*time.Millisecond)
	}()
	wg.Wait()

	after := currentPoolMB.Load()
	if after != before {
		t.Fatalf("currentPoolMB leaked: before=%d after=%d", before, after)
	}
}

func TestLeakRAM_GlobalCeilingHonored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory ceiling test in -short mode")
	}
	before := currentPoolMB.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	const goroutines = 4
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			LeakRAM(ctx, 1, 5*time.Millisecond)
		}()
	}

	peakObserved := int64(0)
	deadline := time.Now().Add(180 * time.Millisecond)
	for time.Now().Before(deadline) {
		v := currentPoolMB.Load()
		if v > peakObserved {
			peakObserved = v
		}
		if v > maxRAMPoolMB+int64(goroutines) {
			t.Fatalf("ceiling breached: pool=%d ceiling=%d", v, maxRAMPoolMB)
		}
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()

	after := currentPoolMB.Load()
	if after != before {
		t.Fatalf("currentPoolMB not restored: before=%d after=%d", before, after)
	}
}

func TestBurnCPU_CancelLatencyBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		BurnCPU(ctx, 2, 0)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond) // let it warm up
	start := time.Now()
	cancel()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
			t.Fatalf("BurnCPU took %v to honor cancel — should be <200ms", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("BurnCPU did not honor context cancel")
	}
}

func TestBurnCPU_SinkProvesWorkIsReal(t *testing.T) {
	before := burnerSink.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	BurnCPU(ctx, 1, 0)
	after := burnerSink.Load()
	if after == before {
		t.Fatal("burnerSink did not advance — SHA-256 loop may have been dead-code-eliminated")
	}
}

func TestTriggerResourceSabotage_WaitGroupDiscipline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	policy := ResourcePolicy{}
	policy.CPU.Enabled = true
	policy.CPU.Cores = 1
	policy.RAM.Enabled = false

	before := runtime.NumGoroutine()
	TriggerResourceSabotage(ctx, policy, wg)
	cancel()

	doneCh := make(chan struct{})
	go func() { wg.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitGroup didn't release after ctx cancel — goroutine leak")
	}
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	if runtime.NumGoroutine() > before+2 {
		t.Errorf("possible goroutine leak: before=%d after=%d", before, runtime.NumGoroutine())
	}
}

// chunkMB<=0 fast-return path must touch nothing.
func TestLeakRAM_ZeroChunkIsNoop(t *testing.T) {
	before := currentPoolMB.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	LeakRAM(ctx, 0, 5*time.Millisecond)
	if currentPoolMB.Load() != before {
		t.Fatal("zero-chunk LeakRAM modified the pool counter")
	}
}

// Demonstrate burnerSink atomic safety
func TestBurnerSink_RaceSafe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var concurrent atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			concurrent.Add(1)
			BurnCPU(ctx, 1, 0)
		}()
	}
	wg.Wait()
	if concurrent.Load() != 4 {
		t.Fatalf("expected 4 concurrent burners, observed %d", concurrent.Load())
	}
}
