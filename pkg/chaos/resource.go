package chaos

import (
	"context"
	"crypto/sha256"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
)

// maxRAMPoolMB caps total RAM allocated by LeakRAM across all policies.
const maxRAMPoolMB int64 = 4096

// burnerInnerIterations bounds how many SHA-256 ops we run between context checks.
const burnerInnerIterations = 1024

var burnerSink atomic.Uint64

// currentPoolMB tracks total live LeakRAM allocation in megabytes.
var currentPoolMB atomic.Int64

var systemPageSize = func() int {
	if ps := os.Getpagesize(); ps > 0 {
		return ps
	}
	return 4096
}()

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

	inner := threshold
	if inner <= 0 || inner > burnerInnerIterations {
		inner = burnerInnerIterations
	}

	var wg sync.WaitGroup
	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()

			payload := []byte("pastaay-cpu-vector-XXXXXXXX")
			ctr := seed
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				var local uint64
				for j := 0; j < inner; j++ {
					ctr++
					// Embed the counter
					payload[len(payload)-8] = byte(ctr)
					payload[len(payload)-7] = byte(ctr >> 8)
					payload[len(payload)-6] = byte(ctr >> 16)
					payload[len(payload)-5] = byte(ctr >> 24)
					payload[len(payload)-4] = byte(ctr >> 32)
					payload[len(payload)-3] = byte(ctr >> 40)
					payload[len(payload)-2] = byte(ctr >> 48)
					payload[len(payload)-1] = byte(ctr >> 56)
					sum := sha256.Sum256(payload)
					local ^= uint64(sum[0]) | uint64(sum[8])<<8 |
						uint64(sum[16])<<16 | uint64(sum[24])<<24
				}
				burnerSink.Add(local + 1)
			}
		}(uint64(i) * 0x9E3779B97F4A7C15)
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
	pool := make([][]byte, 0, 16)

	// addedMB tracks how much this goroutine added to currentPoolMB.
	var addedMB int64
	defer func() {
		if addedMB > 0 {
			currentPoolMB.Add(-addedMB)
		}
		pool = nil
		runtime.GC()
	}()

	allocate := func() bool {
		want := int64(chunkMB)
		for {
			cur := currentPoolMB.Load()
			if cur+want > maxRAMPoolMB {
				log.Printf("[Pastaay-Resource] RAM ceiling %dMB reached, refusing new chunk (%dMB)", maxRAMPoolMB, chunkMB)
				return false
			}
			if currentPoolMB.CompareAndSwap(cur, cur+want) {
				break
			}
		}
		chunk := make([]byte, chunkSize)

		for i := 0; i < chunkSize; i += systemPageSize {
			chunk[i] = 1
		}
		pool = append(pool, chunk)
		addedMB += want
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

// TriggerResourceSabotage dispatches resource chaos under the given context.
func TriggerResourceSabotage(ctx context.Context, policy ResourcePolicy, wg *sync.WaitGroup) {
	if policy.CPU.Enabled {
		_, span := tracing.StartChaosSpan(ctx, "pastaay.resource.cpu", "CPU Burner Activated", "burner")
		telemetry.EmitInfo("resource", "CPU Burner Activated", map[string]interface{}{
			"cores": policy.CPU.Cores, "threshold": policy.CPU.ThrottleThreshold,
		}, span)

		if wg != nil {
			wg.Add(1)
		}
		if policy.CPU.Duration > 0 {
			cpuCtx, cancel := context.WithTimeout(ctx, policy.CPU.Duration)
			go func() {
				defer cancel()
				if wg != nil {
					defer wg.Done()
				}
				BurnCPU(cpuCtx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
			}()
		} else {
			go func() {
				if wg != nil {
					defer wg.Done()
				}
				BurnCPU(ctx, policy.CPU.Cores, policy.CPU.ThrottleThreshold)
			}()
		}
	}

	if policy.RAM.Enabled {
		_, span := tracing.StartChaosSpan(ctx, "pastaay.resource.ram", "RAM Leaker Activated", "leaker")
		telemetry.EmitInfo("resource", "RAM Leaker Activated", map[string]interface{}{
			"chunk_mb": policy.RAM.ChunkMB, "interval": policy.RAM.Interval.String(),
		}, span)

		if wg != nil {
			wg.Add(1)
		}
		if policy.RAM.Duration > 0 {
			ramCtx, cancel := context.WithTimeout(ctx, policy.RAM.Duration)
			go func() {
				defer cancel()
				if wg != nil {
					defer wg.Done()
				}
				LeakRAM(ramCtx, policy.RAM.ChunkMB, policy.RAM.Interval)
			}()
		} else {
			go func() {
				if wg != nil {
					defer wg.Done()
				}
				LeakRAM(ctx, policy.RAM.ChunkMB, policy.RAM.Interval)
			}()
		}
	}
}

// MonitorAndTrigger runs as a daemon to handle resource sabotage lifecycle safely.
func MonitorAndTrigger(ctx context.Context, mgr *config.Manager) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastResourceHash uint64
	var activeCancel context.CancelFunc
	var activeWG *sync.WaitGroup

	cleanup := func() {
		if activeCancel != nil {
			activeCancel()
			activeCancel = nil
		}
		if activeWG != nil {
			activeWG.Wait()
			activeWG = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			cleanup()
			return
		case <-ticker.C:
			policies := mgr.GetActivePolicies("resource")

			if len(policies) == 0 {
				if activeCancel != nil {
					cleanup()
					lastResourceHash = 0
				}
				continue
			}

			var combinedHash uint64
			for _, p := range policies {
				combinedHash ^= p.PolicyHash
			}

			if combinedHash == lastResourceHash {
				continue
			}

			cleanup()
			lastResourceHash = combinedHash

			chaosCtx, cancel := context.WithCancel(ctx)
			activeCancel = cancel
			activeWG = &sync.WaitGroup{}

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
				TriggerResourceSabotage(chaosCtx, rp, activeWG)
			}
		}
	}
}
