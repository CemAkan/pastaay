package chaos

import (
	"context"
	"crypto/sha256"
	"runtime"
	"sync"
	"time"
)

// ResourcePolicy holds the configuration for OS-level resource sabotage.
type ResourcePolicy struct {
	CPU struct {
		Enabled           bool
		Cores             int
		Duration          time.Duration
		ThrottleThreshold int
	}
	RAM struct {
		Enabled  bool
		ChunkMB  int
		Interval time.Duration
		Duration time.Duration
	}
}

// BurnCPU locks the processor by bypassing compiler optimizations with local sinks.
func BurnCPU(ctx context.Context, cores int, threshold int) {
	if cores <= 0 {
		cores = runtime.NumCPU()
	}
	if threshold <= 0 {
		threshold = 100000
	}

	var wg sync.WaitGroup
	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload := []byte("pastaay-cpu-vector")
			var localSink [32]byte
			for {
				for j := 0; j < threshold; j++ {
					localSink = sha256.Sum256(payload)
				}
				_ = localSink
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
		}()
	}
	wg.Wait()
}

// LeakRAM exhausts physical RAM by evading the OS Demand Paging mechanism.
func LeakRAM(ctx context.Context, chunkMB int, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	chunkSize := chunkMB * 1024 * 1024
	var pool [][]byte

	for {
		select {
		case <-ctx.Done():
			pool = nil
			runtime.GC()
			return
		case <-ticker.C:
			chunk := make([]byte, chunkSize)
			// Page-forcing
			for i := 0; i < chunkSize; i += 4096 {
				chunk[i] = 1
			}
			pool = append(pool, chunk)
		}
	}
}

// TriggerResourceSabotage asynchronously starts resource exhaustion based on policy.
func TriggerResourceSabotage(policy ResourcePolicy) {
	if policy.CPU.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), policy.CPU.Duration)
		go func() {
			defer cancel()
			BurnCPU(ctx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
		}()
	}

	if policy.RAM.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), policy.RAM.Duration)
		go func() {
			defer cancel()
			LeakRAM(ctx, policy.RAM.ChunkMB, policy.RAM.Interval)
		}()
	}
}
