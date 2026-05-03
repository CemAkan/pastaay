package chaos

import (
	"context"
	"crypto/sha256"
	"runtime"
	"time"
)

// Sink prevents the Go compiler from optimizing away our CPU-burning loops.
var CPUSink [32]byte

// ResourcePolicy holds the configuration for OS-level resource sabotage.
type ResourcePolicy struct {
	CPU struct {
		Enabled  bool
		Cores    int           // 0 means all available cores (runtime.NumCPU)
		Duration time.Duration // Duration of the chaos
	}
	RAM struct {
		Enabled  bool
		ChunkMB  int           // Megabytes to allocate per tick
		Interval time.Duration // Allocation interval
		Duration time.Duration // Duration of the chaos
	}
}

// BurnCPU locks the processor to 100% by bypassing compiler optimizations.
func BurnCPU(ctx context.Context, cores int) {
	if cores <= 0 {
		cores = runtime.NumCPU()
	}

	for i := 0; i < cores; i++ {
		go func() {
			// Heavy payload to keep the ALU busy
			dummyPayload := []byte("pastaay-strict-cpu-sabotage-vector")
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Cryptographic operation assigned to a global sink
					CPUSink = sha256.Sum256(dummyPayload)
				}
			}
		}()
	}
}

// LeakRAM exhausts physical RAM by evading the OS Demand Paging mechanism.
func LeakRAM(ctx context.Context, chunkMB int, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	chunkSize := chunkMB * 1024 * 1024

	// Local pool ensures isolation
	var pool [][]byte

	for {
		select {
		case <-ctx.Done():
			// CHAOS ENDED
			pool = nil
			runtime.GC()
			return
		case <-ticker.C:
			// Allocate memory
			chunk := make([]byte, chunkSize)

			// Write 1 byte to every 4KB block
			for i := 0; i < chunkSize; i += 4096 {
				chunk[i] = 1
			}

			// Append to local pool to evade premature garbage collection
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
			BurnCPU(ctx, policy.CPU.Cores)
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
