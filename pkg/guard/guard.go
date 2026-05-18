package guard

import (
	"fmt"

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

	for _, p := range cfg.Policies {
		if p.LatencyChance == 0 && p.ErrorChance == 0 && !p.DropConnection && p.RAMChunkMB == 0 && p.ThrottleThreshold == 0 {
			continue
		}

		scopeWeight := 0.4
		if p.Target == "all" || p.Target == "database" || p.Target == "*" || p.Target == "" {
			scopeWeight = 1.0
			res.Issues = append(res.Issues, fmt.Sprintf("[%s] Global Target: Exposes entire '%s' infrastructure layer.", p.Name, p.Type))
		}

		policyRisk := 0.3 * scopeWeight
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
