package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/grpcchaos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	C_Reset         = "\x1b[0m"
	C_Glitch        = "\x1b[38;5;198m"
	C_Neural        = "\x1b[38;5;51m"
	C_Void          = "\x1b[38;5;160m"
	C_Stable        = "\x1b[38;5;82m"
	C_Yellow        = "\x1b[38;5;226m"
	C_Gray          = "\x1b[90m"
	C_Startup       = "\033c\x1b[?25l"
	JitterThreshold = 40 * time.Millisecond
	RenderFPS       = 20 * time.Millisecond
)

const logo = `
 ██████╗  █████╗ ███████╗████████╗ █████╗  █████╗ ██╗   ██╗
 ██╔══██╗██╔══██╗██╔════╝╚══██╔══╝██╔══██╗██╔══██╗╚██╗ ██╔╝
 ██████╔╝███████║███████╗   ██║   ███████║███████║ ╚████╔╝ 
 ██╔═══╝ ██╔══██║╚════██║   ██║   ██╔══██║██╔══██║  ╚██╔╝  
 ██║     ██║  ██║███████║   ██║   ██║  ██║██║  ██║   ██║   
 ╚═╝     ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   `

var parsedLogo []string

func init() { parsedLogo = strings.Split(strings.Trim(logo, "\n"), "\n") }

type ProbeState struct {
	mu         sync.RWMutex
	ProbeStart time.Time
	LastJitter time.Duration
	IsVoid     bool
	IsActive   bool
}

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig("pastaay.yaml")
	if err != nil {
		log.Fatalf("[FATAL] config load: %v", err)
	}
	log.SetOutput(io.Discard)
	fmt.Print(C_Startup)

	mgr := config.NewManager(cfg)
	if stop, werr := config.WatchConfig("pastaay.yaml", mgr.Update); werr == nil {
		defer stop()
	}

	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			return
		}
		s := grpc.NewServer(grpc.UnaryInterceptor(grpcchaos.UnaryInterceptor(mgr)))
		_ = s.Serve(lis)
	}()

	state := &ProbeState{}
	go func() {
		for {
			state.mu.Lock()
			state.IsActive, state.ProbeStart = true, time.Now()
			state.mu.Unlock()

			err := simulateNeuralDrift(context.Background(), mgr)
			elapsed := time.Since(state.ProbeStart)

			state.mu.Lock()
			state.IsActive, state.LastJitter, state.IsVoid = false, elapsed, (err != nil)
			state.mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
	}()

	frame := 0.0
	for {
		var screen strings.Builder
		screen.WriteString("\x1b[H\x1b[K\n\x1b[K\n")

		state.mu.RLock()
		isVoid, active := state.IsVoid, state.IsActive
		elapsed := state.LastJitter
		if active {
			elapsed = time.Since(state.ProbeStart)
		}
		state.mu.RUnlock()

		isGlitch := elapsed > JitterThreshold
		renderProLogo(&screen, isVoid, isGlitch)
		screen.WriteString(C_Gray + " ━" + strings.Repeat("━", 60) + C_Reset + "\x1b[K\n")
		drawVortexField(&screen, frame, isVoid, isGlitch)
		screen.WriteString(C_Gray + " ━" + strings.Repeat("━", 60) + C_Reset + "\x1b[K\n")
		renderProStatus(&screen, isVoid, isGlitch, elapsed, int(frame))

		fmt.Print(screen.String())
		if isGlitch {
			frame += 0.05
		} else {
			frame += 0.25
		}
		time.Sleep(RenderFPS)
	}
}

func renderProLogo(sb *strings.Builder, isVoid, isGlitch bool) {
	for _, line := range parsedLogo {
		prefix := "  "
		if isGlitch && rand.Float64() > 0.8 {
			prefix = strings.Repeat(" ", rand.IntN(3))
		}
		if isVoid {
			sb.WriteString(prefix + C_Void + corruptLine(line) + C_Reset + "\x1b[K\n")
		} else {
			sb.WriteString(prefix + C_Neural + line + C_Reset + "\x1b[K\n")
		}
	}
}

func drawVortexField(sb *strings.Builder, f float64, isVoid, isGlitch bool) {
	height, width := 7, 60
	for y := 0; y < height; y++ {
		sb.WriteString("  ")
		for x := 0; x < width; x++ {
			val := math.Sin(f*0.4 + float64(x)*0.15 + float64(y)*0.35)
			if isVoid {
				if rand.Float64() > 0.92 {
					sb.WriteString(C_Void + "░" + C_Reset)
				} else {
					sb.WriteString(" ")
				}
			} else {
				if math.Abs(val) > 0.94 {
					if isGlitch {
						sb.WriteString(C_Yellow + "⚡" + C_Reset)
					} else {
						sb.WriteString(C_Neural + "◈" + C_Reset)
					}
				} else if math.Abs(val) > 0.8 {
					sb.WriteString(C_Stable + "◦" + C_Reset)
				} else if math.Abs(val) > 0.6 {
					sb.WriteString(C_Gray + "·" + C_Reset)
				} else {
					sb.WriteString(" ")
				}
			}
		}
		sb.WriteString("\x1b[K\n")
	}
}

func renderProStatus(sb *strings.Builder, isVoid, isGlitch bool, d time.Duration, f int) {
	sb.WriteString("  ")
	if isVoid {
		fmt.Fprintf(sb, "%s[VOID]   %-22s ✘ SIGNAL_LOST%s\x1b[K\n", C_Void, "DISCONNECTED", C_Reset)
	} else if isGlitch {
		fmt.Fprintf(sb, "%s[GLITCH] %-22s ⚠ JITTER: %-10v%s\x1b[K\n", C_Glitch, "STUTTER_ACTIVE", d.Round(time.Millisecond), C_Reset)
	} else {
		spin := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[f%10]
		fmt.Fprintf(sb, "%s[STABLE] %s %-18s ✔ FLOW_SYNC_OK%s\x1b[K\n", C_Stable, spin, "VORTEX_STREAM", C_Reset)
	}
}

func corruptLine(s string) string {
	r := []rune(s)
	for i := range r {
		if r[i] != ' ' && rand.Float64() < 0.25 {
			r[i] = []rune{'?', '#', '░', '▒'}[rand.IntN(4)]
		}
	}
	return string(r)
}

func simulateNeuralDrift(ctx context.Context, mgr *config.Manager) error {
	policies := mgr.GetActivePolicies("grpc")
	if len(policies) == 0 {
		return nil
	}
	p := policies[0]
	if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
		time.Sleep(p.LatencyDuration)
	}
	if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
		return status.Error(codes.Internal, p.ErrorBody)
	}
	return nil
}
