package guard

import (
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

func TestAnalyze_NilOrEmptyConfigIsSafe(t *testing.T) {
	if got := Analyze(nil); got.Status != "SAFE" {
		t.Errorf("nil config should yield SAFE, got %s", got.Status)
	}
	if got := Analyze(&config.PastaayConfig{}); got.Status != "SAFE" {
		t.Errorf("empty config should yield SAFE, got %s", got.Status)
	}
}

func TestAnalyze_ScoreScalesWithSeverity(t *testing.T) {
	low := Analyze(&config.PastaayConfig{
		EnableDefaultIgnored: true,
		Policies: []config.Policy{
			{Name: "tiny", Type: "http", Target: "/a", ErrorChance: 0.05},
		},
	})
	high := Analyze(&config.PastaayConfig{
		EnableDefaultIgnored: true,
		Policies: []config.Policy{
			{Name: "global-kill", Type: "http", Target: "all", ErrorChance: 1.0, DropConnection: true},
		},
	})
	if low.Score >= high.Score {
		t.Fatalf("low(%d) must be < high(%d)", low.Score, high.Score)
	}
}

func TestAnalyze_ScoreBoundedAndCritical(t *testing.T) {
	pols := make([]config.Policy, 0, 50)
	for i := 0; i < 50; i++ {
		pols = append(pols, config.Policy{
			Name: "p", Type: "http", Target: "all", ErrorChance: 1.0, DropConnection: true,
		})
	}
	res := Analyze(&config.PastaayConfig{EnableDefaultIgnored: true, Policies: pols})
	if res.Score < 0 || res.Score > 100 {
		t.Fatalf("Score out of bounds: %d", res.Score)
	}
	if res.Status != "CRITICAL" {
		t.Fatalf("expected CRITICAL status, got %s", res.Status)
	}
}

func TestAnalyze_5sLatencyTriggersThreadPoolWarning(t *testing.T) {
	res := Analyze(&config.PastaayConfig{
		EnableDefaultIgnored: true,
		Policies: []config.Policy{
			{Name: "slow", Type: "http", Target: "/", LatencyChance: 0.5, LatencyDuration: 6 * time.Second},
		},
	})
	found := false
	for _, msg := range res.Issues {
		if containsAll(msg, []string{"slow", "thread-pool"}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("5s latency policy must surface thread-pool warning, got %v", res.Issues)
	}
}

func TestAnalyze_ResourceOOMWarning(t *testing.T) {
	res := Analyze(&config.PastaayConfig{
		EnableDefaultIgnored: true,
		Policies: []config.Policy{
			{Name: "oom", Type: "resource", Target: "host", RAMChunkMB: 2048},
		},
	})
	found := false
	for _, m := range res.Issues {
		if containsAll(m, []string{"OOM"}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("RAMChunkMB>=1024 must trigger OOM Risk warning, got %v", res.Issues)
	}
}

func TestAnalyze_CollisionDetected(t *testing.T) {
	res := Analyze(&config.PastaayConfig{
		EnableDefaultIgnored: true,
		Policies: []config.Policy{
			{Name: "p1", Type: "http", Target: "all", ErrorChance: 0.5},
			{Name: "p2", Type: "http", Target: "all", LatencyChance: 0.5, LatencyDuration: time.Second},
		},
	})
	found := false
	for _, m := range res.Issues {
		if containsAll(m, []string{"Collision"}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("overlapping policies must flag collision, got %v", res.Issues)
	}
}

func containsAll(s string, needles []string) bool {
	for _, n := range needles {
		if !contains(s, n) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
