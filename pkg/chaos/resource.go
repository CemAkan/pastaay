package chaos

import (
	"context"
	"crypto/sha256"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
)

// maxRAMPoolMB is a global per-process ceiling to prevent the engine from self-OOMing when policies request unbounded RAM allocations.
const maxRAMPoolMB int64 = 4096

// currentPoolMB tracks total live LeakRAM allocation in megabytes.
var currentPoolMB int64

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

// BurnCPU locks the processor by bypassing compiler optimizations.
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

				runtime.KeepAlive(localSink)
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

// LeakRAM exhausts physical RAM via page-forcing, capped by maxRAMPoolMB.
func LeakRAM(ctx context.Context, chunkMB int, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if chunkMB <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	chunkSize := chunkMB * 1024 * 1024
	var pool [][]byte

	defer func() {
		atomic.AddInt64(&currentPoolMB, -int64(len(pool)*chunkMB))
		pool = nil
		runtime.GC()
	}()

	// DRY allocation logic with hard ceiling enforcement.
	allocate := func() bool {
		if atomic.LoadInt64(&currentPoolMB)+int64(chunkMB) > maxRAMPoolMB {
			log.Printf("[Pastaay-Resource] RAM ceiling %dMB reached, refusing new chunk (%dMB)", maxRAMPoolMB, chunkMB)
			return false
		}
		chunk := make([]byte, chunkSize)
		// Page-forcing
		for i := 0; i < chunkSize; i += 4096 {
			chunk[i] = 1
		}
		pool = append(pool, chunk)
		atomic.AddInt64(&currentPoolMB, int64(chunkMB))
		return true
	}

	allocate()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			allocate()
		}
	}
}

// TriggerResourceSabotage safely dispatches resource chaos.
func TriggerResourceSabotage(ctx context.Context, policy ResourcePolicy) {
	if policy.CPU.Enabled {

		_, span := tracing.StartChaosSpan(ctx, "pastaay.resource.cpu", "CPU Burner Activated", "burner")

		telemetry.EmitInfo("resource", "CPU Burner Activated", map[string]interface{}{
			"cores": policy.CPU.Cores, "threshold": policy.CPU.ThrottleThreshold,
		}, span)

		if policy.CPU.Duration > 0 {
			cpuCtx, cancel := context.WithTimeout(ctx, policy.CPU.Duration)
			go func() {
				defer cancel()
				BurnCPU(cpuCtx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
			}()
		} else {
			go BurnCPU(ctx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
		}
	}

	if policy.RAM.Enabled {

		_, span := tracing.StartChaosSpan(ctx, "pastaay.resource.ram", "RAM Leaker Activated", "leaker")

		telemetry.EmitInfo("resource", "RAM Leaker Activated", map[string]interface{}{
			"chunk_mb": policy.RAM.ChunkMB, "interval": policy.RAM.Interval.String(),
		}, span)

		if policy.RAM.Duration > 0 {
			ramCtx, cancel := context.WithTimeout(ctx, policy.RAM.Duration)
			go func() {
				defer cancel()
				LeakRAM(ramCtx, policy.RAM.ChunkMB, policy.RAM.Interval)
			}()
		} else {
			go LeakRAM(ctx, policy.RAM.ChunkMB, policy.RAM.Interval)
		}
	}
}

// MonitorAndTrigger runs as a daemon to handle resource sabotage lifecycle safely.
func MonitorAndTrigger(ctx context.Context, mgr *config.Manager) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastResourceHash uint64
	var activeCancel context.CancelFunc

	for {
		select {
		case <-ctx.Done():
			if activeCancel != nil {
				activeCancel()
			}
			return
		case <-ticker.C:
			policies := mgr.GetActivePolicies("resource")

			// Immediate kill-switch on rollback
			if len(policies) == 0 {
				if activeCancel != nil {
					activeCancel()
					activeCancel = nil
					lastResourceHash = 0
				}
				continue
			}

			var combinedHash uint64
			for _, p := range policies {
				combinedHash = (combinedHash<<1 | combinedHash>>63) ^ p.PolicyHash
			}

			if combinedHash != lastResourceHash {
				// Kill previous chaos if any
				if activeCancel != nil {
					activeCancel()
				}

				lastResourceHash = combinedHash

				var chaosCtx context.Context
				chaosCtx, activeCancel = context.WithCancel(ctx)

				for _, p := range policies {
					rp := ResourcePolicy{}
					if p.LatencyDuration > 0 {
						rp.CPU.Enabled = true
						rp.CPU.Duration = p.LatencyDuration
						rp.CPU.Cores = 0
						rp.CPU.ThrottleThreshold = p.ThrottleThreshold
					}
					if p.RAMChunkMB > 0 {
						rp.RAM.Enabled = true
						rp.RAM.ChunkMB = p.RAMChunkMB
						rp.RAM.Interval = p.RAMInterval
						rp.RAM.Duration = p.LatencyDuration
					}
					TriggerResourceSabotage(chaosCtx, rp)
				}
			}
		}
	}
}
