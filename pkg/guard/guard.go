package guard

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

type PlanResult struct {
	TotalRisk float64  `json:"total_risk"`
	Status    string   `json:"status"`
	Issues    []string `json:"issues"`
	Score     int      `json:"score"`
}

func Analyze(cfg *config.PastaayConfig) PlanResult {
	res := PlanResult{Issues: make([]string, 0)}

	if cfg == nil || len(cfg.Policies) == 0 {
		return PlanResult{Status: "SAFE", Score: 0, TotalRisk: 0.0, Issues: []string{}}
	}

	systemSurvival := 1.0

	if !cfg.EnableDefaultIgnored {
		res.Issues = append(res.Issues, "CORE GUARD DISABLED: System base vulnerability increased.")
		systemSurvival *= (1.0 - 0.15)
	}

	targets := make(map[string]string)

	for _, p := range cfg.Policies {
		if p.LatencyChance == 0 && p.ErrorChance == 0 && !p.DropConnection && p.RAMChunkMB == 0 && p.ThrottleThreshold == 0 {
			continue
		}

		scopeWeight := 0.4
		if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, "database") || p.Target == "*" || p.Target == "" {
			scopeWeight = 1.0
			res.Issues = append(res.Issues, fmt.Sprintf("[%s] Global Target: Exposes entire '%s' infrastructure layer.", p.Name, p.Type))
		}

		key := strings.ToLower(p.Type) + ":" + strings.ToLower(p.Target)
		if orig, exists := targets[key]; exists {
			res.Issues = append(res.Issues, fmt.Sprintf("[%s] Collision: Overlaps with '%s'. Cascading failure probability increased.", p.Name, orig))
			scopeWeight = math.Min(1.0, scopeWeight+0.2)
		}
		targets[key] = p.Name

		maxSeverity := 0.0

		if p.DropConnection {
			maxSeverity = math.Max(maxSeverity, 1.0)
			res.Issues = append(res.Issues, fmt.Sprintf("[%s] Hard TCP Drop: Triggers immediate network circuit-breakers.", p.Name))
		}

		if p.ErrorChance > 0 {
			maxSeverity = math.Max(maxSeverity, p.ErrorChance)
		}

		if p.LatencyChance > 0 {
			latMultiplier := 0.3
			if p.LatencyDuration >= 5*time.Second {
				latMultiplier = 1.0
				res.Issues = append(res.Issues, fmt.Sprintf("[%s] 5s+ Timeout: Causes severe thread-pool exhaustion.", p.Name))
			} else if p.LatencyDuration >= 1*time.Second {
				latMultiplier = 0.6
			}
			latSeverity := p.LatencyChance * latMultiplier
			maxSeverity = math.Max(maxSeverity, latSeverity)
		}

		if p.Type == "resource" {
			resSeverity := 0.0
			if p.RAMChunkMB >= 1024 {
				resSeverity = 0.9
				res.Issues = append(res.Issues, fmt.Sprintf("[%s] OOM Risk: >1GB physical RAM allocation detected.", p.Name))
			} else if p.RAMChunkMB >= 256 {
				resSeverity = 0.5
			} else if p.RAMChunkMB > 0 {
				resSeverity = 0.2
			}

			if p.ThrottleThreshold >= 100000 {
				resSeverity = math.Max(resSeverity, 0.8)
				res.Issues = append(res.Issues, fmt.Sprintf("[%s] CPU Lock: High cryptographic throttle ceiling.", p.Name))
			} else if p.ThrottleThreshold > 0 {
				resSeverity = math.Max(resSeverity, 0.4)
			}

			maxSeverity = math.Max(maxSeverity, resSeverity)
		}

		policyRisk := maxSeverity * scopeWeight

		if policyRisk > 0.95 {
			policyRisk = 0.95
		}
		systemSurvival *= (1.0 - policyRisk)
	}

	finalRisk := 1.0 - systemSurvival
	res.TotalRisk = finalRisk
	res.Score = int(finalRisk * 100.0)

	switch {
	case res.Score >= 75:
		res.Status = "CRITICAL"
	case res.Score >= 50:
		res.Status = "HIGH"
	case res.Score >= 25:
		res.Status = "ELEVATED"
	default:
		res.Status = "SAFE"
	}

	return res
}
