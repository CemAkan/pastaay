package chaos

import (
	"context"
	"crypto/sha256"
	"runtime"
	"sync"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
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

// LeakRAM exhausts physical RAM via page-forcing.
func LeakRAM(ctx context.Context, chunkMB int, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	chunkSize := chunkMB * 1024 * 1024
	var pool [][]byte

	// DRY allocation logic
	allocate := func() {
		chunk := make([]byte, chunkSize)
		// Page-forcing
		for i := 0; i < chunkSize; i += 4096 {
			chunk[i] = 1
		}
		pool = append(pool, chunk)
	}

	allocate()

	for {
		select {
		case <-ctx.Done():
			// Immediate cleanup
			pool = nil
			runtime.GC()
			return
		case <-ticker.C:
			allocate()
		}
	}
}

// TriggerResourceSabotage safely dispatches resource chaos.
func TriggerResourceSabotage(ctx context.Context, policy ResourcePolicy) {
	if policy.CPU.Enabled {
		if policy.CPU.Duration > 0 {
			cpuCtx, cancel := context.WithTimeout(ctx, policy.CPU.Duration)
			go func() {
				defer cancel()
				BurnCPU(cpuCtx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
			}()
		} else {
			// Zero-duration bypass to prevent immediate suicide
			go BurnCPU(ctx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
		}
	}

	if policy.RAM.Enabled {
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
