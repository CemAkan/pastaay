package telemetry

import (
	"sync"
	"testing"
)

func TestRingBuffer_WrapsCleanly(t *testing.T) {
	mu.Lock()
	buf = [256]LogEntry{}
	head, size = 0, 0
	mu.Unlock()

	for i := 0; i < 1000; i++ {
		Emit("test", "x", "y")
	}
	snap := Snapshot()
	if len(snap) != 256 {
		t.Fatalf("expected 256 entries after wrap, got %d", len(snap))
	}
}

// concurrent Emit + Snapshot must not race.
func TestRingBuffer_ConcurrentReadWriteSafe(t *testing.T) {
	mu.Lock()
	buf = [256]LogEntry{}
	head, size = 0, 0
	mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				Emit("a", "b", "c")
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = Snapshot()
			}
		}()
	}
	wg.Wait()
}

func TestEmitError_NilSpanIsSafe(t *testing.T) {
	EmitError("http", "/x", "msg", "{}", nil)
	EmitInfo("http", "ok", nil, nil)
}
